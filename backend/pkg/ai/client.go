// Package ai wraps the LLM provider behind one small client.
//
// The provider is Groq by default (free tier, OpenAI-compatible wire
// format), with OpenAI as a drop-in fallback when only OPENAI_API_KEY
// is set. Every call is metered: token usage is priced, accumulated
// against a monthly budget, and written to ai_logs. When the budget is
// spent the circuit breaker opens and calls fail fast instead of
// quietly running up a bill.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	groqBaseURL   = "https://api.groq.com/openai/v1"
	openAIBaseURL = "https://api.openai.com/v1"

	// defaultGroqModel is current as of Aug 2026. The llama-3.1-70b
	// generation the day-8 guide names has been decommissioned.
	defaultGroqModel   = "llama-3.3-70b-versatile"
	defaultOpenAIModel = "gpt-4o-mini"

	// Vision models are separate: the default chat model is text-only,
	// so image QA overrides the model per request. Qwen is the
	// image-capable model Groq currently serves; it is also a
	// reasoning model, hence stripReasoning below.
	defaultGroqVisionModel   = "qwen/qwen3.6-27b"
	defaultOpenAIVisionModel = "gpt-4o-mini"

	// defaultCostLimitUSD is the fallback monthly budget. Sscanf leaves
	// the target untouched on a malformed value, so an unparseable or
	// zero AI_MONTHLY_COST_LIMIT_USD must fall back here rather than
	// wedging the breaker permanently open at $0.
	defaultCostLimitUSD = 50.0

	maxAttempts = 3

	// maxRetryAfter caps how long a provider's Retry-After hint can
	// stall one request.
	maxRetryAfter = 20 * time.Second
)

// ErrDisabled is returned when no provider API key is configured.
// AI features degrade to no-ops rather than taking the process down.
var ErrDisabled = errors.New("ai: no provider configured (set GROQ_API_KEY or OPENAI_API_KEY)")

// ErrCostLimit is returned when the monthly budget is exhausted.
var ErrCostLimit = errors.New("ai: monthly cost limit exceeded, circuit breaker open")

var httpClient = &http.Client{Timeout: 60 * time.Second}

// pricing is USD per million tokens, keyed by model prefix.
// Approximate published list prices — good enough to keep a budget.
var pricing = map[string]struct{ input, output float64 }{
	"llama-3.3-70b":    {0.59, 0.79},
	"llama-3.1-8b":     {0.05, 0.08},
	"llama-4-scout":    {0.11, 0.34},
	"llama-4-maverick": {0.20, 0.60},
	"gpt-4o-mini":      {0.15, 0.60},
	"gpt-4o":           {2.50, 10.00},
}

// fallbackPricing is used for unrecognised models. Deliberately on the
// expensive side so an unknown model trips the breaker early rather
// than late.
var fallbackPricing = struct{ input, output float64 }{1.00, 2.00}

// Client is a metered LLM client. Safe for concurrent use.
type Client struct {
	db      *pgxpool.Pool
	apiKey  string
	baseURL string
	model   string

	visionModel  string
	costLimitUSD float64

	// costMicro is month-to-date spend in micro-dollars (1e-6 USD),
	// matching the NUMERIC(10,6) precision of ai_logs.cost_usd.
	// An integer keeps the accumulator race-free.
	costMicro atomic.Int64
	requests  atomic.Int64

	// mu guards the reseed of the month-to-date accumulator
	mu     sync.Mutex
	period string // "2006-01" of the currently loaded totals
}

// NewClient builds a client from the environment. It never panics: with
// no API key it returns a disabled client whose calls fail with
// ErrDisabled, so the API still boots without AI configured.
func NewClient(db *pgxpool.Pool) *Client {
	c := &Client{db: db, costLimitUSD: defaultCostLimitUSD}

	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		c.apiKey = key
		c.baseURL = groqBaseURL
		c.model = envOr("GROQ_MODEL", defaultGroqModel)
		c.visionModel = envOr("GROQ_VISION_MODEL", defaultGroqVisionModel)
	} else if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		c.apiKey = key
		c.baseURL = openAIBaseURL
		c.model = envOr("OPENAI_MODEL", defaultOpenAIModel)
		c.visionModel = envOr("OPENAI_VISION_MODEL", defaultOpenAIVisionModel)
	}

	if raw := os.Getenv("AI_MONTHLY_COST_LIMIT_USD"); raw != "" {
		if limit, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil && limit > 0 {
			c.costLimitUSD = limit
		}
	}

	return c
}

// Enabled reports whether a provider is configured.
func (c *Client) Enabled() bool { return c.apiKey != "" }

// Model returns the chat model in use (empty when disabled).
func (c *Client) Model() string { return c.model }

// VisionModel returns the model to set on Request.Model when the
// request carries images.
func (c *Client) VisionModel() string { return c.visionModel }

// modelFor resolves the model a request runs against.
func (c *Client) modelFor(override string) string {
	if override != "" {
		return override
	}
	return c.model
}

// Image is one image attached to a request, for vision-capable models.
type Image struct {
	// DataURI is a self-contained "data:image/jpeg;base64,..." value.
	// Sending bytes rather than a URL keeps the provider from having to
	// reach our storage host.
	DataURI string
}

// Request is one completion. Feature, TenantID and ReferenceID are not
// sent to the provider — they label the ai_logs row this call writes.
type Request struct {
	Feature     string // 'copy_gen', 'recommendations', 'auto_reply', 'image_qa'
	TenantID    string // optional
	ReferenceID string // optional: product_id, ticket_id, ...

	System string
	User   string
	Images []Image

	// Model overrides the client's default for this call. Set it from
	// Client.VisionModel() when Images is non-empty — the default chat
	// model cannot accept images.
	Model string

	// ReasoningEffort is only meaningful for reasoning models. "none"
	// suppresses the <think> trace, which on the vision model cuts a
	// reply from ~900 completion tokens to ~80 — the difference
	// between fitting the free tier's token-per-minute budget and
	// exhausting it on a single image.
	ReasoningEffort string

	MaxTokens   int
	Temperature float64
}

// Complete runs one chat completion and returns the message content.
func (c *Client) Complete(ctx context.Context, req Request) (string, error) {
	if !c.Enabled() {
		return "", ErrDisabled
	}
	if err := c.checkBudget(ctx); err != nil {
		return "", err
	}

	model := c.modelFor(req.Model)

	start := time.Now()
	body, usage, err := c.post(ctx, req, model)
	latency := time.Since(start)

	cost := priceOf(model, usage)
	if err == nil {
		c.costMicro.Add(int64(cost * 1_000_000))
		c.requests.Add(1)
	}
	c.log(req, model, usage, cost, latency, err)

	if err != nil {
		return "", err
	}
	return stripReasoning(body), nil
}

// stripReasoning removes the <think>...</think> trace that reasoning
// models emit before their answer. Left in place it becomes part of a
// published product description, and its braces defeat JSON
// extraction.
func stripReasoning(s string) string {
	for {
		start := strings.Index(s, "<think>")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "</think>")
		if end < 0 {
			// Truncated mid-thought (hit the token ceiling): everything
			// from the opening tag on is reasoning, not answer.
			return strings.TrimSpace(s[:start])
		}
		s = s[:start] + s[start+end+len("</think>"):]
	}
	return strings.TrimSpace(s)
}

// CompleteJSON runs a completion that must return a single JSON object,
// and strips any markdown fencing or prose the model wraps it in.
func (c *Client) CompleteJSON(ctx context.Context, req Request) (string, error) {
	req.System = strings.TrimSpace(req.System) +
		"\n\nIMPORTANT: Respond with a single valid JSON object and nothing else. " +
		"No markdown, no code fences, no preamble."

	raw, err := c.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	obj := ExtractJSON(raw)
	if obj == "" {
		return "", fmt.Errorf("ai: no JSON object in response: %.120q", raw)
	}
	return obj, nil
}

// ExtractJSON pulls the first balanced JSON object out of a model
// response. Small models routinely ignore "JSON only" and wrap the
// object in ```json fences or a sentence of explanation, which is the
// single most common cause of parse failures.
func ExtractJSON(s string) string {
	depth, start, inString, escaped := 0, -1, false, false

	for i, r := range s {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// literal text, no structural meaning
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			depth--
			if depth == 0 && start >= 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// Stats reports month-to-date usage, read from ai_logs so the numbers
// survive a restart.
type Stats struct {
	TotalRequests      int64
	TotalCostUSD       float64
	CostLimitUSD       float64
	CostRemainingUSD   float64
	Model              string
	Enabled            bool
	CircuitBreakerOpen bool
}

// Stats returns current usage. It reseeds from the database so a
// long-lived process reflects work done by the worker too.
func (c *Client) Stats(ctx context.Context) Stats {
	if err := c.reseed(ctx, time.Now().UTC().Format("2006-01")); err != nil {
		// Fall through to in-memory totals — stats must not error out
		// the dashboard just because the log table is unreachable.
		_ = err
	}

	spent := float64(c.costMicro.Load()) / 1_000_000
	return Stats{
		TotalRequests:      c.requests.Load(),
		TotalCostUSD:       spent,
		CostLimitUSD:       c.costLimitUSD,
		CostRemainingUSD:   math0(c.costLimitUSD - spent),
		Model:              c.model,
		Enabled:            c.Enabled(),
		CircuitBreakerOpen: c.Enabled() && spent >= c.costLimitUSD,
	}
}

// ── internals ────────────────────────────────────────────────

type chatRequest struct {
	Model           string        `json:"model"`
	Messages        []chatMessage `json:"messages"`
	MaxTokens       int           `json:"max_tokens,omitempty"`
	Temperature     float64       `json:"temperature"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
}

// Content is either a plain string or a []contentPart (vision), which
// is why it is typed as any rather than string.
type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type contentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *imageURLPart `json:"image_url,omitempty"`
}

type imageURLPart struct {
	URL string `json:"url"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// post sends the completion, retrying on rate limits and transient
// upstream failures. Groq's free tier allows ~30 requests/minute and a
// copy generation burst is six calls, so 429s are routine rather than
// exceptional.
func (c *Client) post(ctx context.Context, req Request, model string) (string, usage, error) {
	payload := chatRequest{
		Model:           model,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
	}
	if payload.MaxTokens == 0 {
		payload.MaxTokens = 1024
	}
	if req.System != "" {
		payload.Messages = append(payload.Messages, chatMessage{
			Role: "system", Content: req.System,
		})
	}

	if len(req.Images) == 0 {
		payload.Messages = append(payload.Messages, chatMessage{
			Role: "user", Content: req.User,
		})
	} else {
		parts := []contentPart{{Type: "text", Text: req.User}}
		for _, img := range req.Images {
			parts = append(parts, contentPart{
				Type:     "image_url",
				ImageURL: &imageURLPart{URL: img.DataURI},
			})
		}
		payload.Messages = append(payload.Messages, chatMessage{
			Role: "user", Content: parts,
		})
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", usage{}, fmt.Errorf("ai: encode request: %w", err)
	}

	var lastErr error
	var retryAfter time.Duration

	for attempt := range maxAttempts {
		if attempt > 0 {
			// Exponential backoff, unless the provider told us exactly
			// how long to wait. Token-per-minute limits need the best
			// part of a minute, which no backoff curve would guess.
			delay := time.Duration(1<<(attempt-1)) * time.Second
			if retryAfter > 0 {
				delay = retryAfter
			}
			select {
			case <-ctx.Done():
				return "", usage{}, ctx.Err()
			case <-time.After(delay):
			}
		}

		content, u, retryable, wait, err := c.attempt(ctx, encoded)
		if err == nil {
			return content, u, nil
		}
		lastErr = err
		retryAfter = wait
		if !retryable {
			break
		}
	}
	return "", usage{}, lastErr
}

// parseRetryAfter reads the Retry-After header, which the provider
// sends in seconds. Capped so a long limit window cannot pin a request
// open for minutes.
func parseRetryAfter(h http.Header) time.Duration {
	raw := strings.TrimSpace(h.Get("Retry-After"))
	if raw == "" {
		return 0
	}
	secs, err := strconv.ParseFloat(raw, 64)
	if err != nil || secs <= 0 {
		return 0
	}
	wait := time.Duration(secs * float64(time.Second))
	if wait > maxRetryAfter {
		return maxRetryAfter
	}
	return wait
}

// attempt makes a single HTTP call. The bool reports whether a retry
// could plausibly succeed; the duration is the provider's own
// Retry-After hint, zero when it gave none.
func (c *Client) attempt(ctx context.Context, encoded []byte) (string, usage, bool, time.Duration, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return "", usage{}, false, 0, fmt.Errorf("ai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", usage{}, true, 0, fmt.Errorf("ai: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", usage{}, true, 0, fmt.Errorf("ai: read response: %w", err)
	}

	retryAfter := parseRetryAfter(resp.Header)

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", usage{}, resp.StatusCode >= 500, retryAfter,
			fmt.Errorf("ai: decode response (HTTP %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := http.StatusText(resp.StatusCode)
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", parsed.Usage, retryable, retryAfter,
			fmt.Errorf("ai: provider error (HTTP %d): %s", resp.StatusCode, msg)
	}

	if len(parsed.Choices) == 0 {
		return "", parsed.Usage, true, retryAfter, errors.New("ai: provider returned no choices")
	}

	return parsed.Choices[0].Message.Content, parsed.Usage, false, 0, nil
}

func priceOf(model string, u usage) float64 {
	price := fallbackPricing
	for prefix, p := range pricing {
		// Groq prefixes vision models with the vendor
		// ("meta-llama/llama-4-scout-..."), so match on substring
		// rather than requiring the model id to start with the key.
		if strings.Contains(model, prefix) {
			price = p
			break
		}
	}
	return (float64(u.PromptTokens)*price.input +
		float64(u.CompletionTokens)*price.output) / 1_000_000
}

// checkBudget opens the circuit breaker once month-to-date spend
// reaches the limit.
func (c *Client) checkBudget(ctx context.Context) error {
	period := time.Now().UTC().Format("2006-01")
	if err := c.reseed(ctx, period); err != nil {
		// Budget state unknown. Allowing the call is the lesser evil:
		// the limit is a cost guard, not a correctness guarantee, and
		// the log write below still records the spend.
		_ = err
	}

	spent := float64(c.costMicro.Load()) / 1_000_000
	if spent >= c.costLimitUSD {
		return fmt.Errorf("%w ($%.4f / $%.2f)", ErrCostLimit, spent, c.costLimitUSD)
	}
	return nil
}

// reseed loads month-to-date totals from ai_logs the first time this
// period is seen. That makes the breaker survive restarts (Railway
// restarts on every deploy) and reset itself at the month boundary
// without a scheduled job.
func (c *Client) reseed(ctx context.Context, period string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.period == period {
		return nil
	}
	if c.db == nil {
		c.period = period
		return nil
	}

	var costUSD float64
	var count int64
	err := c.db.QueryRow(ctx, `
        SELECT COALESCE(SUM(cost_usd), 0), COUNT(*)
        FROM ai_logs
        WHERE created_at >= date_trunc('month', NOW())
    `).Scan(&costUSD, &count)
	if err != nil {
		return fmt.Errorf("ai: load month-to-date spend: %w", err)
	}

	c.costMicro.Store(int64(costUSD * 1_000_000))
	c.requests.Store(count)
	c.period = period
	return nil
}

// log records every call — successes and failures — for cost tracking,
// evaluation and debugging.
func (c *Client) log(req Request, model string, u usage, cost float64, latency time.Duration, callErr error) {
	if c.db == nil {
		return
	}

	var errMsg *string
	if callErr != nil {
		msg := callErr.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
		errMsg = &msg
	}

	// Detached context: the log must land even when the caller's
	// context was cancelled (which is exactly when failures happen).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.db.Exec(ctx, `
        INSERT INTO ai_logs
            (tenant_id, feature, model, input_tokens, output_tokens,
             cost_usd, latency_ms, success, error_message, reference_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `,
		nullableUUID(req.TenantID),
		req.Feature,
		model,
		u.PromptTokens,
		u.CompletionTokens,
		cost,
		latency.Milliseconds(),
		callErr == nil,
		errMsg,
		nullableText(req.ReferenceID),
	)
	if err != nil {
		fmt.Printf("ai: failed to write ai_logs row (feature=%s): %v\n", req.Feature, err)
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func math0(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
