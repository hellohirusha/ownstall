package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/pkg/ai"
)

// ImageQAService grades product photos with a vision model so listings
// can be held back before they reach the storefront.
type ImageQAService struct {
	DB *pgxpool.Pool
	AI *ai.Client
}

// imagePassThreshold is the score at or above which a photo is
// considered good enough to publish.
const imagePassThreshold = 0.6

// maxImageBytes caps the download. Vision requests carry the image
// inline as base64, so an oversized photo is a slow, expensive request
// rather than a better analysis.
const maxImageBytes = 4 << 20 // 4 MB

var imageClient = &http.Client{Timeout: 30 * time.Second}

// ImageQAResult is the verdict on one photo. The json tags map the
// model's response; ImageURL and Passed are filled in by us.
type ImageQAResult struct {
	QualityScore float64  `json:"score"`
	Issues       []string `json:"issues"`
	Suggestion   string   `json:"suggestion"`
	ImageURL     string   `json:"-"`
	Passed       bool     `json:"-"`
}

const imageQAPrompt = `Analyse this product photo for e-commerce use.

Judge it on:
- Resolution: enough detail to show the product?
- Lighting: well lit, no harsh shadows or blown highlights?
- Background: clean (white, neutral) or a deliberate lifestyle setting?
- Sharpness: is the product in focus?
- Framing: centred and well cropped?
- Presentation: does it look professional?

Use only these issue codes: blurry, low_resolution, poor_lighting,
cluttered_background, bad_cropping, watermark, text_overlay,
not_a_product_photo

Respond with JSON only:
{"score": 0.85, "issues": ["poor_lighting"], "suggestion": "one specific fix"}`

// AnalyzeProductImage downloads a photo, has the vision model grade
// it, and stores the verdict.
func (s *ImageQAService) AnalyzeProductImage(
	ctx context.Context, tenantID, productID, imageURL string,
) (*ImageQAResult, error) {
	if !s.AI.Enabled() {
		return nil, ai.ErrDisabled
	}

	data, contentType, err := downloadImage(ctx, downscale(imageURL))
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}

	dataURI := fmt.Sprintf("data:%s;base64,%s", contentType,
		base64.StdEncoding.EncodeToString(data))

	response, err := s.AI.CompleteJSON(ctx, ai.Request{
		Feature:     "image_qa",
		TenantID:    tenantID,
		ReferenceID: productID,
		Model:       s.AI.VisionModel(),
		User:        imageQAPrompt,
		Images:      []ai.Image{{DataURI: dataURI}},
		Temperature: 0.2,
		// Without this the model spends its whole budget reasoning and
		// returns a truncated think block with no JSON in it.
		ReasoningEffort: "none",
		MaxTokens:       400,
	})
	if err != nil {
		return nil, fmt.Errorf("image analysis failed: %w", err)
	}

	var result ImageQAResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse image QA response: %w", err)
	}

	result.QualityScore = clamp01(result.QualityScore)
	result.Passed = result.QualityScore >= imagePassThreshold
	result.ImageURL = imageURL
	if result.Issues == nil {
		result.Issues = []string{}
	}

	if err := s.store(ctx, tenantID, productID, imageURL, &result); err != nil {
		// The verdict is still useful to the caller even unsaved
		fmt.Printf("image_qa: failed to store result for product %s: %v\n", productID, err)
	}

	return &result, nil
}

// AnalyzeProductImages grades every photo on a product and returns the
// results in image order.
func (s *ImageQAService) AnalyzeProductImages(
	ctx context.Context, tenantID, productID string,
) ([]*ImageQAResult, error) {
	rows, err := s.DB.Query(ctx, `
        SELECT pi.url
        FROM product_images pi
        JOIN products p ON p.id = pi.product_id
        WHERE pi.product_id = $1 AND p.tenant_id = $2
        ORDER BY pi.position ASC
    `, productID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list product images: %w", err)
	}

	var urls []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan image url: %w", err)
		}
		urls = append(urls, url)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read product images: %w", err)
	}

	results := make([]*ImageQAResult, 0, len(urls))
	for _, url := range urls {
		result, err := s.AnalyzeProductImage(ctx, tenantID, productID, url)
		if err != nil {
			fmt.Printf("image_qa: %s failed: %v\n", url, err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

func (s *ImageQAService) store(
	ctx context.Context, tenantID, productID, imageURL string, result *ImageQAResult,
) error {
	_, err := s.DB.Exec(ctx, `
        INSERT INTO image_qa_results
            (tenant_id, product_id, image_url, quality_score, issues, suggestion, passed)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (product_id, image_url) DO UPDATE
        SET quality_score = EXCLUDED.quality_score,
            issues        = EXCLUDED.issues,
            suggestion    = EXCLUDED.suggestion,
            passed        = EXCLUDED.passed,
            created_at    = NOW()
    `, tenantID, productID, imageURL, result.QualityScore,
		result.Issues, // pgx encodes []string as text[] natively
		result.Suggestion, result.Passed)
	if err != nil {
		return fmt.Errorf("failed to store image QA result: %w", err)
	}
	return nil
}

// downscale asks Cloudinary for a smaller copy before we send the
// image to the model. Vision input is priced and rate-limited by
// token count, and token count scales with pixels — a full-resolution
// product shot can cost more than a whole page of text while telling
// the model nothing extra about focus, lighting or framing.
//
// Non-Cloudinary URLs are returned untouched.
func downscale(rawURL string) string {
	const marker = "/image/upload/"
	if !strings.Contains(rawURL, "res.cloudinary.com") {
		return rawURL
	}
	idx := strings.Index(rawURL, marker)
	if idx < 0 {
		return rawURL
	}

	// c_limit only shrinks: an image already under 768px is untouched
	return rawURL[:idx+len(marker)] + "w_768,c_limit,q_auto/" + rawURL[idx+len(marker):]
}

// downloadImage fetches the photo and reports its content type.
func downloadImage(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := imageClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d downloading image", resp.StatusCode)
	}

	// LimitReader guards against a hostile or mis-sized image; +1 byte
	// so an image exactly at the cap is detectable as over it.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxImageBytes {
		return nil, "", fmt.Errorf("image exceeds %d byte limit", maxImageBytes)
	}

	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("URL is not an image (content type %q)", contentType)
	}

	return data, contentType, nil
}
