package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/pkg/ai"
)

// CopyGeneratorService writes product descriptions with an LLM, scores
// them, and publishes the best one automatically when it clears the
// quality bar.
type CopyGeneratorService struct {
	DB *pgxpool.Pool
	AI *ai.Client

	// Recommendations, when set, re-indexes a product's embedding after
	// its description is replaced. Without it, auto-published copy
	// silently leaves the similarity vector describing the old text.
	Recommendations *RecommendationService
}

// autoPublishThreshold is the quality score at or above which copy
// goes live with no human review.
const autoPublishThreshold = 0.85

// GeneratedCopy is one description variant.
type GeneratedCopy struct {
	Body            string
	QualityScore    float64
	ToneLabel       string
	WordCount       int
	WillAutoPublish bool
}

// tones are the variants generated for every product.
var tones = []struct {
	label  string
	prompt string
}{
	{
		label: "professional",
		prompt: "Write a professional, benefit-focused product description. " +
			"Emphasize quality, materials, and use cases. Clear, concise language. Max 120 words.",
	},
	{
		label: "casual",
		prompt: "Write a friendly, conversational product description. " +
			"Speak directly to the customer, show enthusiasm, keep it fun. Max 120 words.",
	},
	{
		label: "punchy",
		prompt: "Write a short, punchy product description optimized for conversion. " +
			"Lead with the strongest benefit. Under 80 words.",
	},
}

func (s *CopyGeneratorService) setTenantContext(ctx context.Context, tenantID string) error {
	_, err := s.DB.Exec(ctx,
		"SELECT set_config('app.current_tenant_id', $1, true)",
		tenantID,
	)
	return err
}

// GenerateProductCopy produces three variants concurrently, scores
// them in one comparative pass, and auto-publishes the winner when it
// scores at or above the threshold.
func (s *CopyGeneratorService) GenerateProductCopy(
	ctx context.Context, tenantID, productID string,
) ([]*GeneratedCopy, error) {
	if !s.AI.Enabled() {
		return nil, ai.ErrDisabled
	}
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	product, err := s.loadProduct(ctx, tenantID, productID)
	if err != nil {
		return nil, err
	}

	copies, err := s.generateVariants(ctx, tenantID, productID, product)
	if err != nil {
		return nil, err
	}

	best := copies[0]
	for _, c := range copies {
		if c.QualityScore > best.QualityScore {
			best = c
		}
	}

	if best.WillAutoPublish {
		if err := s.autoPublish(ctx, tenantID, productID, best); err != nil {
			// The copy is still returned for manual review
			fmt.Printf("copy_gen: auto-publish failed for product %s: %v\n", productID, err)
		}
	}

	return copies, nil
}

// GenerateCopyForText generates and scores variants for a product that
// exists only as a description — used by the evaluation script, which
// runs against a fixed test set rather than the catalogue, and never
// publishes.
func (s *CopyGeneratorService) GenerateCopyForText(
	ctx context.Context, name, description string, tags []string, price float64,
) ([]*GeneratedCopy, error) {
	if !s.AI.Enabled() {
		return nil, ai.ErrDisabled
	}

	product := &copyProduct{
		Name:        name,
		Description: description,
		Tags:        tags,
		BasePrice:   price,
	}
	return s.generateVariants(ctx, "", "", product)
}

// generateVariants runs the three tones concurrently and scores them.
// It touches no database, so it is shared by the catalogue path and
// the evaluation harness.
func (s *CopyGeneratorService) generateVariants(
	ctx context.Context, tenantID, productID string, product *copyProduct,
) ([]*GeneratedCopy, error) {
	systemPrompt := product.systemPrompt()

	// Variants are generated concurrently but failures are collected
	// per-variant: two good descriptions are a useful result, so one
	// provider hiccup must not discard the whole batch.
	results := make([]*GeneratedCopy, len(tones))
	errs := make([]error, len(tones))

	var wg sync.WaitGroup
	for i, tone := range tones {
		wg.Add(1)
		go func() {
			defer wg.Done()

			body, err := s.AI.Complete(ctx, ai.Request{
				Feature:     "copy_gen",
				TenantID:    tenantID,
				ReferenceID: productID,
				System:      systemPrompt,
				User:        tone.prompt,
				Temperature: 0.7,
				MaxTokens:   400,
			})
			if err != nil {
				errs[i] = fmt.Errorf("tone %s: %w", tone.label, err)
				return
			}

			body = strings.TrimSpace(body)
			results[i] = &GeneratedCopy{
				Body:      body,
				ToneLabel: tone.label,
				WordCount: len(strings.Fields(body)),
			}
		}()
	}
	wg.Wait()

	copies := make([]*GeneratedCopy, 0, len(tones))
	for _, r := range results {
		if r != nil {
			copies = append(copies, r)
		}
	}
	if len(copies) == 0 {
		return nil, fmt.Errorf("copy generation failed: %w", firstErr(errs))
	}

	// One comparative scoring call rather than one per variant: it
	// halves the request count against Groq's free-tier rate limit,
	// and rating variants side by side is more consistent than rating
	// each in isolation.
	s.scoreCopies(ctx, tenantID, productID, product.Name, copies)

	return copies, nil
}

// ── product context ──────────────────────────────────────────

type copyProduct struct {
	Name        string
	Description string
	BasePrice   float64
	Tags        []string
	ImageAlts   []string
}

func (s *CopyGeneratorService) loadProduct(ctx context.Context, tenantID, productID string) (*copyProduct, error) {
	var p copyProduct
	err := s.DB.QueryRow(ctx, `
        SELECT name, COALESCE(description, ''), base_price, tags
        FROM products
        WHERE id = $1 AND tenant_id = $2
    `, productID, tenantID).Scan(&p.Name, &p.Description, &p.BasePrice, &p.Tags)
	if err != nil {
		return nil, fmt.Errorf("product %s not found: %w", productID, err)
	}

	// Alt text, not image URLs: a text model cannot see a URL, and
	// feeding it one invites invented detail about the photo.
	rows, err := s.DB.Query(ctx, `
        SELECT alt_text FROM product_images
        WHERE product_id = $1 AND alt_text IS NOT NULL AND alt_text <> ''
        ORDER BY position ASC LIMIT 3
    `, productID)
	if err != nil {
		return &p, nil // context is optional; the description is not
	}
	defer rows.Close()

	for rows.Next() {
		var alt string
		if err := rows.Scan(&alt); err != nil {
			continue
		}
		p.ImageAlts = append(p.ImageAlts, alt)
	}

	return &p, nil
}

func (p *copyProduct) systemPrompt() string {
	var b strings.Builder
	b.WriteString("You are an expert e-commerce copywriter. ")
	b.WriteString("You write product descriptions that turn browsers into buyers.\n\n")
	fmt.Fprintf(&b, "Product name: %s\n", p.Name)
	fmt.Fprintf(&b, "Price: $%.2f\n", p.BasePrice)

	if len(p.Tags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(p.Tags, ", "))
	}
	if p.Description != "" {
		fmt.Fprintf(&b, "Existing description: %s\n", p.Description)
	}
	if len(p.ImageAlts) > 0 {
		fmt.Fprintf(&b, "Product photos show: %s\n", strings.Join(p.ImageAlts, "; "))
	}

	b.WriteString("\nRules:\n")
	b.WriteString("- Only claim what the details above support. Never invent materials, dimensions, or certifications.\n")
	b.WriteString("- Write the description only. No headings, no preamble, no quotation marks.\n")

	return b.String()
}

// ── scoring ──────────────────────────────────────────────────

// scoreCopies rates every variant in a single call. On any failure the
// variants keep a neutral 0.5, which is below the auto-publish
// threshold — an unscored variant is never published unreviewed.
func (s *CopyGeneratorService) scoreCopies(
	ctx context.Context, tenantID, productID, productName string, copies []*GeneratedCopy,
) {
	for _, c := range copies {
		c.QualityScore = 0.5
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Rate each product description for %q on a 0.0 to 1.0 scale.\n\n", productName)
	for i, c := range copies {
		fmt.Fprintf(&b, "Variant %d (%s):\n%s\n\n", i+1, c.ToneLabel, c.Body)
	}
	// Sub-scores rather than one overall number. Asked for a single
	// score, the model returns the same confident value for every
	// variant, which makes the auto-publish threshold meaningless.
	// Rating four narrow criteria and averaging them produces spread,
	// and makes a low score explainable.
	b.WriteString(`Rate each variant on four criteria, each 0.0 to 1.0:
- accuracy: every claim is supported by the product details. Deduct
  heavily for invented materials, dimensions, certifications or uses.
- conversion: gives a concrete reason to buy rather than generic praise.
- clarity: plain, readable, no filler.
- length: suits the format; deduct for padding or for being too thin.

Be critical and use the full range. A competent but unremarkable
description should score around 0.6-0.7 — reserve scores above 0.9 for
copy you would publish to a paying customer with no edits. Do not give
every variant the same score; rank them against each other.

Respond with JSON only:
{"scores": [{"variant": 1, "accuracy": 0.9, "conversion": 0.7,
"clarity": 0.8, "length": 0.7, "reason": "brief"}]}`)

	resp, err := s.AI.CompleteJSON(ctx, ai.Request{
		Feature:     "copy_gen",
		TenantID:    tenantID,
		ReferenceID: productID,
		System:      "You are a strict content quality evaluator. You always respond with valid JSON.",
		User:        b.String(),
		Temperature: 0.2,
		MaxTokens:   400,
	})
	if err != nil {
		fmt.Printf("copy_gen: scoring failed for product %s: %v\n", productID, err)
		return
	}

	var parsed struct {
		Scores []struct {
			Variant    int     `json:"variant"`
			Accuracy   float64 `json:"accuracy"`
			Conversion float64 `json:"conversion"`
			Clarity    float64 `json:"clarity"`
			Length     float64 `json:"length"`
			Reason     string  `json:"reason"`
		} `json:"scores"`
	}
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		fmt.Printf("copy_gen: could not parse scores for product %s: %v\n", productID, err)
		return
	}

	for _, sc := range parsed.Scores {
		idx := sc.Variant - 1
		if idx < 0 || idx >= len(copies) {
			continue
		}

		// Accuracy is weighted double: copy that invents facts is a
		// liability no amount of persuasiveness offsets.
		weighted := (2*clamp01(sc.Accuracy) +
			clamp01(sc.Conversion) +
			clamp01(sc.Clarity) +
			clamp01(sc.Length)) / 5

		copies[idx].QualityScore = clamp01(weighted)
		copies[idx].WillAutoPublish = copies[idx].QualityScore >= autoPublishThreshold
	}
}

// ── publishing ───────────────────────────────────────────────

func (s *CopyGeneratorService) autoPublish(
	ctx context.Context, tenantID, productID string, best *GeneratedCopy,
) error {
	_, err := s.DB.Exec(ctx, `
        UPDATE products
        SET description      = $1,
            ai_description   = $1,
            ai_quality_score = $2,
            ai_generated_at  = NOW(),
            updated_at       = NOW()
        WHERE id = $3 AND tenant_id = $4
    `, best.Body, best.QualityScore, productID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to publish copy: %w", err)
	}

	fmt.Printf("copy_gen: auto-published %s copy for product %s (score %.2f)\n",
		best.ToneLabel, productID, best.QualityScore)

	// The description just changed, so the similarity vector built
	// from the old text is now stale.
	if s.Recommendations != nil {
		if err := s.Recommendations.IndexProduct(ctx, tenantID, productID); err != nil {
			fmt.Printf("copy_gen: re-index after publish failed for %s: %v\n", productID, err)
		}
	}

	return nil
}

// ── A/B tracking ─────────────────────────────────────────────

// RecordCopyImpression notes that a session saw a particular copy
// variant, so conversion can be attributed to it later.
func (s *CopyGeneratorService) RecordCopyImpression(
	ctx context.Context, tenantID, productID, variant, sessionID string,
) error {
	_, err := s.DB.Exec(ctx, `
        INSERT INTO ab_test_events
            (tenant_id, product_id, variant, session_id, event_type)
        VALUES ($1, $2, $3, $4, 'impression')
    `, tenantID, productID, variant, sessionID)
	if err != nil {
		return fmt.Errorf("failed to record impression: %w", err)
	}
	return nil
}

// RecordCopyConversion marks the session's impression as converted.
func (s *CopyGeneratorService) RecordCopyConversion(
	ctx context.Context, tenantID, productID, sessionID string,
) error {
	_, err := s.DB.Exec(ctx, `
        UPDATE ab_test_events
        SET converted_at = NOW()
        WHERE tenant_id = $1
          AND product_id = $2
          AND session_id = $3
          AND event_type = 'impression'
          AND converted_at IS NULL
    `, tenantID, productID, sessionID)
	if err != nil {
		return fmt.Errorf("failed to record conversion: %w", err)
	}
	return nil
}

// ── dashboard helpers ────────────────────────────────────────

// ProductRef is a minimal product identity for dashboard pickers.
type ProductRef struct {
	ID   string
	Name string
}

// ProductsMissingCopy lists products that have never had AI copy
// generated — the queue the admin page offers to work through.
func (s *CopyGeneratorService) ProductsMissingCopy(ctx context.Context, tenantID string) ([]*ProductRef, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	rows, err := s.DB.Query(ctx, `
        SELECT id, name FROM products
        WHERE tenant_id = $1 AND ai_description IS NULL AND status <> 'archived'
        ORDER BY created_at DESC
    `, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list products missing copy: %w", err)
	}
	defer rows.Close()

	refs := make([]*ProductRef, 0)
	for rows.Next() {
		var r ProductRef
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		refs = append(refs, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read products: %w", err)
	}

	return refs, nil
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func firstErr(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
