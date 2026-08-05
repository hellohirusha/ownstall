//go:build ignore

// AI evaluation harness.
//
//	cd backend && go run scripts/ai_eval.go
//
// Runs the copy generator over a fixed test set and reports pass rate,
// quality, latency and cost. Writes ai_eval_report.json.
//
// On human ratings: this harness does NOT invent them. Every case
// ships with HumanRating 0, meaning unrated. Read the generated copy
// in the report, set HumanRating (1-5) on the cases in testDataset
// below, and re-run to compare the model's self-scoring against human
// judgement. Until then the report says "unrated" rather than quoting
// a number nobody produced — a model marking its own homework is
// exactly what an evaluation is supposed to catch.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/hellohirusha/ownstall/internal/services"
	"github.com/hellohirusha/ownstall/pkg/ai"
	"github.com/hellohirusha/ownstall/pkg/database"
)

// TestCase is one product the generator must write copy for.
type TestCase struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Price       float64  `json:"price"`
	Category    string   `json:"category"`

	// Facts that must NOT appear unless supported by the input. Each
	// is a term the copy has no basis to claim.
	ForbiddenClaims []string `json:"forbidden_claims"`

	// HumanRating is 1-5, or 0 for unrated.
	HumanRating float64 `json:"human_rating"`
}

// testDataset is deliberately small and hand-written. Extend it as
// real products reveal real failure modes.
var testDataset = []TestCase{
	{
		Name: "Custom Die-Cut Vinyl Sticker", Price: 3.50, Category: "stickers",
		Description: "3x3 inch waterproof die-cut vinyl sticker, custom shape",
		Tags:        []string{"stickers", "vinyl", "waterproof"},
		// Nothing in the input supports a dishwasher or UV claim
		ForbiddenClaims: []string{"dishwasher", "uv-resistant", "uv resistant"},
	},
	{
		Name: "Holographic Sticker Sheet", Price: 12.00, Category: "stickers",
		Description:     "Sheet of 12 holographic designs, weather resistant",
		Tags:            []string{"stickers", "holographic"},
		ForbiddenClaims: []string{"biodegradable", "recycled"},
	},
	{
		Name: "Matte Sticker Bundle", Price: 8.00, Category: "stickers",
		Description:     "Matte finish sticker bundle, scratch resistant vinyl",
		Tags:            []string{"stickers", "matte"},
		ForbiddenClaims: []string{"glossy", "waterproof"},
	},
	{
		Name: "Unisex Heavyweight T-Shirt", Price: 28.00, Category: "apparel",
		Description:     "100% cotton, screen printed logo, unisex fit",
		Tags:            []string{"apparel", "cotton"},
		ForbiddenClaims: []string{"organic", "polyester", "moisture-wicking"},
	},
	{
		Name: "Heavyweight Cotton Hoodie", Price: 48.00, Category: "apparel",
		Description:     "Unisex heavyweight cotton hoodie with screen printed chest logo",
		Tags:            []string{"apparel", "hoodie"},
		ForbiddenClaims: []string{"waterproof", "fleece-lined", "organic"},
	},
	{
		Name: "Enamel Pin - Cat Astronaut", Price: 11.00, Category: "pins",
		Description:     "Hard enamel pin, 1.25 inch, double rubber clutch backing",
		Tags:            []string{"pins", "enamel"},
		ForbiddenClaims: []string{"magnetic", "glow in the dark", "limited edition"},
	},
	{
		Name: "A2 Riso Art Print", Price: 35.00, Category: "prints",
		Description:     "Two-colour risograph print on 200gsm recycled paper, A2",
		Tags:            []string{"prints", "riso"},
		ForbiddenClaims: []string{"framed", "signed", "numbered"},
	},
	{
		Name: "Digital Brush Pack", Price: 15.00, Category: "digital",
		Description:     "40 procreate brushes for inking and texture, instant download",
		Tags:            []string{"digital", "procreate"},
		ForbiddenClaims: []string{"photoshop", "refund", "physical"},
	},
	{
		Name: "Ceramic Mug 11oz", Price: 18.00, Category: "homeware",
		Description:     "11oz white ceramic mug with wraparound print",
		Tags:            []string{"homeware", "mug"},
		ForbiddenClaims: []string{"microwave safe", "dishwasher safe", "insulated"},
	},
	{
		Name: "Canvas Tote Bag", Price: 22.00, Category: "accessories",
		Description:     "Natural cotton canvas tote, 15x16 inch, screen printed design",
		Tags:            []string{"accessories", "tote"},
		ForbiddenClaims: []string{"waterproof", "leather", "zippered"},
	},
	{
		Name: "Sticker Starter Pack", Price: 6.00, Category: "stickers",
		Description:     "Five assorted small vinyl stickers, mixed designs",
		Tags:            []string{"stickers", "bundle"},
		ForbiddenClaims: []string{"holographic", "custom"},
	},
	{
		Name: "Embroidered Patch", Price: 9.00, Category: "accessories",
		Description:     "3 inch iron-on embroidered patch with merrowed border",
		Tags:            []string{"accessories", "patch"},
		ForbiddenClaims: []string{"velcro", "sew-on only", "reflective"},
	},
}

// EvalResult is the outcome for one test case.
type EvalResult struct {
	TestCase        TestCase `json:"test_case"`
	BestTone        string   `json:"best_tone"`
	BestCopy        string   `json:"best_copy"`
	QualityScore    float64  `json:"quality_score"`
	WordCount       int      `json:"word_count"`
	VariantCount    int      `json:"variant_count"`
	LatencyMs       int64    `json:"latency_ms"`
	PassedThreshold bool     `json:"passed_threshold"`
	// FlaggedTerms lists forbidden terms found in the copy. Matching
	// is literal, so "without the glossy look" flags "glossy" — these
	// are review candidates, not confirmed errors.
	FlaggedTerms []string `json:"flagged_terms"`
	Error        string   `json:"error,omitempty"`
}

// EvalReport is the full run.
type EvalReport struct {
	Timestamp       time.Time `json:"timestamp"`
	Model           string    `json:"model"`
	TotalTestCases  int       `json:"total_test_cases"`
	Completed       int       `json:"completed"`
	Failed          int       `json:"failed"`
	PassRate        float64   `json:"pass_rate"`
	AvgQualityScore float64   `json:"avg_quality_score"`
	AvgLatencyMs    float64   `json:"avg_latency_ms"`
	// FlaggedRate is the share of cases whose copy contained a
	// forbidden term. It is an upper bound on invention, not a
	// hallucination rate: the check is literal substring matching and
	// cannot see negation, so every flag needs a human look before it
	// counts as an error.
	FlaggedRate    float64      `json:"flagged_rate"`
	AvgHumanRating *float64     `json:"avg_human_rating"`
	CostUSD        float64      `json:"cost_usd"`
	CostPerCase    float64      `json:"cost_per_case"`
	Results        []EvalResult `json:"results"`
}

func main() {
	out := flag.String("out", "ai_eval_report.json", "report output path")
	limit := flag.Int("limit", 0, "run only the first N cases (0 = all)")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — using system environment variables")
	}

	db, err := database.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	aiClient := ai.NewClient(db)
	if !aiClient.Enabled() {
		log.Fatal("No AI provider configured (set GROQ_API_KEY or OPENAI_API_KEY)")
	}

	copySvc := &services.CopyGeneratorService{DB: db, AI: aiClient}

	cases := testDataset
	if *limit > 0 && *limit < len(cases) {
		cases = cases[:*limit]
	}

	startStats := aiClient.Stats(context.Background())

	fmt.Println("Ownstall AI Evaluation")
	fmt.Println("======================")
	fmt.Printf("Model: %s\nCases: %d\n\n", aiClient.Model(), len(cases))

	report := EvalReport{
		Timestamp:      time.Now(),
		Model:          aiClient.Model(),
		TotalTestCases: len(cases),
	}

	var totalScore, totalHuman float64
	var totalLatency int64
	var ratedCount, passCount, flaggedCases int

	for i, tc := range cases {
		fmt.Printf("[%2d/%d] %s\n", i+1, len(cases), tc.Name)

		start := time.Now()
		copies, err := copySvc.GenerateCopyForText(
			context.Background(), tc.Name, tc.Description, tc.Tags, tc.Price,
		)
		latency := time.Since(start).Milliseconds()

		if err != nil {
			fmt.Printf("        ERROR: %v\n", err)
			report.Results = append(report.Results, EvalResult{
				TestCase: tc, LatencyMs: latency, Error: err.Error(),
			})
			report.Failed++
			continue
		}

		best := copies[0]
		for _, c := range copies {
			if c.QualityScore > best.QualityScore {
				best = c
			}
		}

		flagged := findFlaggedTerms(best.Body, tc.ForbiddenClaims)
		if len(flagged) > 0 {
			flaggedCases++
		}
		if best.WillAutoPublish {
			passCount++
		}
		if tc.HumanRating > 0 {
			totalHuman += tc.HumanRating / 5.0
			ratedCount++
		}

		totalScore += best.QualityScore
		totalLatency += latency
		report.Completed++

		report.Results = append(report.Results, EvalResult{
			TestCase:        tc,
			BestTone:        best.ToneLabel,
			BestCopy:        best.Body,
			QualityScore:    best.QualityScore,
			WordCount:       best.WordCount,
			VariantCount:    len(copies),
			LatencyMs:       latency,
			PassedThreshold: best.WillAutoPublish,
			FlaggedTerms:    flagged,
		})

		fmt.Printf("        tone=%-12s score=%.2f  %4dms  variants=%d",
			best.ToneLabel, best.QualityScore, latency, len(copies))
		if len(flagged) > 0 {
			fmt.Printf("  FLAGGED: %s", strings.Join(flagged, ", "))
		}
		fmt.Println()
	}

	if report.Completed > 0 {
		n := float64(report.Completed)
		report.PassRate = float64(passCount) / n
		report.AvgQualityScore = totalScore / n
		report.AvgLatencyMs = float64(totalLatency) / n
		report.FlaggedRate = float64(flaggedCases) / n
	}
	if ratedCount > 0 {
		avg := totalHuman / float64(ratedCount)
		report.AvgHumanRating = &avg
	}

	endStats := aiClient.Stats(context.Background())
	report.CostUSD = endStats.TotalCostUSD - startStats.TotalCostUSD
	if report.Completed > 0 {
		report.CostPerCase = report.CostUSD / float64(report.Completed)
	}

	fmt.Println("\n======================")
	fmt.Println("RESULTS")
	fmt.Println("======================")
	fmt.Printf("Completed:            %d/%d (%d failed)\n",
		report.Completed, report.TotalTestCases, report.Failed)
	fmt.Printf("Auto-publish rate:    %.1f%%\n", report.PassRate*100)
	fmt.Printf("Avg quality score:    %.3f\n", report.AvgQualityScore)
	fmt.Printf("Flagged for review:   %.1f%% (literal match, verify each)\n", report.FlaggedRate*100)
	if report.AvgHumanRating != nil {
		fmt.Printf("Avg human rating:     %.3f\n", *report.AvgHumanRating)
	} else {
		fmt.Printf("Avg human rating:     unrated (fill in human_rating and re-run)\n")
	}
	fmt.Printf("Avg latency:          %.0fms\n", report.AvgLatencyMs)
	fmt.Printf("Cost for this run:    $%.6f\n", report.CostUSD)
	fmt.Printf("Cost per case:        $%.6f\n", report.CostPerCase)

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("Failed to encode report: %v", err)
	}
	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		log.Fatalf("Failed to write report: %v", err)
	}
	fmt.Printf("\nReport written to %s\n", *out)
}

// findFlaggedTerms reports which forbidden terms appear in the copy.
// Crude substring matching on purpose: it is a tripwire for obvious
// invention, not a semantic fact-checker. It cannot tell "dishwasher
// safe" from "not dishwasher safe", so a flag means "read this one",
// not "this is wrong".
func findFlaggedTerms(copy string, forbidden []string) []string {
	lower := strings.ToLower(copy)
	var found []string
	for _, term := range forbidden {
		if strings.Contains(lower, strings.ToLower(term)) {
			found = append(found, term)
		}
	}
	return found
}
