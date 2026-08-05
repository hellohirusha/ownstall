package ai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ── response parsing ─────────────────────────────────────────

// Small models routinely ignore "JSON only" and wrap the object in
// fences or a sentence, which is the most common cause of parse
// failures in the scoring path.
func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare object", `{"a":1}`, `{"a":1}`},
		{"surrounding whitespace", "  \n{\"a\":1}\n ", `{"a":1}`},
		{
			"markdown fenced",
			"```json\n{\"scores\":[{\"variant\":1}]}\n```",
			`{"scores":[{"variant":1}]}`,
		},
		{
			"prose before and after",
			`Sure! Here is the result: {"a":1} — let me know if you need more.`,
			`{"a":1}`,
		},
		{"nested objects", `{"a":{"b":{"c":2}}}`, `{"a":{"b":{"c":2}}}`},
		{
			// A brace inside a string literal must not close the object.
			"braces inside a string",
			`{"reason":"use } and { carefully"}`,
			`{"reason":"use } and { carefully"}`,
		},
		{
			"escaped quote inside a string",
			`{"reason":"he said \"} \" and stopped"}`,
			`{"reason":"he said \"} \" and stopped"}`,
		},
		{"first of several objects", `{"a":1} {"b":2}`, `{"a":1}`},
		{"unbalanced", `{"a":1`, ""},
		{"no object at all", `just prose`, ""},
		{"empty", ``, ""},
		// Wrapped in an array, the first balanced object is still what
		// comes back — the callers unmarshal an object, not the array.
		{"object inside an array", `[{"a":1}]`, `{"a":1}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractJSON(tc.in); got != tc.want {
				t.Errorf("ExtractJSON(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Left in place, a reasoning trace becomes part of a published product
// description and its braces defeat JSON extraction.
func TestStripReasoning(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no trace", "A clean description.", "A clean description."},
		{"single block", "<think>hmm, maybe</think>The answer.", "The answer."},
		{"block after content", "The answer.<think>second guess</think>", "The answer."},
		{"two blocks", "<think>a</think>Middle<think>b</think>End", "MiddleEnd"},
		{
			// Hit the token ceiling mid-thought: everything from the
			// opening tag on is reasoning, not answer.
			"truncated, never closed",
			"Partial answer<think>reasoning that never finishes",
			"Partial answer",
		},
		{"whole response is reasoning", "<think>only thinking</think>", ""},
		{"whitespace trimmed", "  <think>x</think>  Answer  ", "Answer"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripReasoning(tc.in); got != tc.want {
				t.Errorf("stripReasoning(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ── cost accounting ──────────────────────────────────────────

func TestPriceOf(t *testing.T) {
	const million = 1_000_000

	cases := []struct {
		name  string
		model string
		usage usage
		want  float64
	}{
		{
			"known chat model",
			"llama-3.3-70b-versatile",
			usage{PromptTokens: million, CompletionTokens: 0},
			0.59,
		},
		{
			"known chat model, output tokens",
			"llama-3.3-70b-versatile",
			usage{PromptTokens: 0, CompletionTokens: million},
			0.79,
		},
		{
			// Groq prefixes vision models with the vendor, so matching
			// must be on substring rather than on a prefix.
			"vendor-prefixed model still matches",
			"meta-llama/llama-4-scout-17b-16e-instruct",
			usage{PromptTokens: million, CompletionTokens: 0},
			0.11,
		},
		{
			// Deliberately expensive so an unknown model trips the
			// breaker early rather than late.
			"unknown model uses fallback pricing",
			"some-model-we-have-never-seen",
			usage{PromptTokens: million, CompletionTokens: million},
			3.00,
		},
		{"zero usage costs nothing", "llama-3.3-70b-versatile", usage{}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := priceOf(tc.model, tc.usage)
			if diff := got - tc.want; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("priceOf(%q, %+v) = %v, want %v", tc.model, tc.usage, got, tc.want)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"absent", "", 0},
		{"whole seconds", "5", 5 * time.Second},
		{"fractional seconds", "2.5", 2500 * time.Millisecond},
		{"padded", "  3  ", 3 * time.Second},
		{"zero", "0", 0},
		{"negative", "-5", 0},
		{"unparseable", "later", 0},
		// A long limit window must not pin one request open for minutes.
		{"above the cap", "600", maxRetryAfter},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.value != "" {
				h.Set("Retry-After", tc.value)
			}
			if got := parseRetryAfter(h); got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

// The breaker is the only thing standing between a runaway loop and a
// real bill, so it must refuse the call before any HTTP request.
func TestComplete_CircuitBreakerOpenAtLimit(t *testing.T) {
	var called atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", baseURL: srv.URL, model: "llama-3.3-70b-versatile"}
	c.costLimitUSD = 1.0
	c.costMicro.Store(1_000_000) // exactly at the limit

	_, err := c.Complete(context.Background(), Request{Feature: "copy_gen", User: "hi"})
	if !errors.Is(err, ErrCostLimit) {
		t.Errorf("error = %v, want ErrCostLimit", err)
	}
	if n := called.Load(); n != 0 {
		t.Errorf("provider was called %d times with the breaker open, want 0", n)
	}
}

func TestStats_ReportsBreakerState(t *testing.T) {
	c := &Client{apiKey: "test-key", model: "llama-3.3-70b-versatile"}
	c.costLimitUSD = 10.0

	c.costMicro.Store(4_000_000) // $4 of $10
	stats := c.Stats(context.Background())
	if stats.CircuitBreakerOpen {
		t.Error("breaker open at $4 of $10, want closed")
	}
	if diff := stats.CostRemainingUSD - 6.0; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("CostRemainingUSD = %v, want 6", stats.CostRemainingUSD)
	}

	c.costMicro.Store(12_000_000) // over budget
	stats = c.Stats(context.Background())
	if !stats.CircuitBreakerOpen {
		t.Error("breaker closed at $12 of $10, want open")
	}
	// Remaining budget must never read negative on a dashboard.
	if stats.CostRemainingUSD != 0 {
		t.Errorf("CostRemainingUSD = %v, want 0", stats.CostRemainingUSD)
	}
}

// A client with no API key must fail fast rather than take the process
// down: AI features degrade to no-ops.
func TestComplete_DisabledClient(t *testing.T) {
	c := &Client{}

	if c.Enabled() {
		t.Error("Enabled() = true on a client with no API key")
	}
	if _, err := c.Complete(context.Background(), Request{User: "hi"}); !errors.Is(err, ErrDisabled) {
		t.Errorf("error = %v, want ErrDisabled", err)
	}
}

// ── provider transport ───────────────────────────────────────

func newTestClient(baseURL string) *Client {
	c := &Client{apiKey: "test-key", baseURL: baseURL, model: "llama-3.3-70b-versatile"}
	c.costLimitUSD = defaultCostLimitUSD
	return c
}

func TestComplete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"<think>weighing it up</think>A tidy description."}}],
			"usage":{"prompt_tokens":100,"completion_tokens":50}
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	got, err := c.Complete(context.Background(), Request{Feature: "copy_gen", User: "describe this"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "A tidy description." {
		t.Errorf("content = %q, want %q", got, "A tidy description.")
	}

	// A successful call must be metered.
	if n := c.requests.Load(); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
	if c.costMicro.Load() == 0 {
		t.Error("cost accumulator did not move after a billed call")
	}
}

// Groq's free tier allows ~30 requests/minute and a copy generation
// burst is six calls, so 429s are routine rather than exceptional.
func TestComplete_RetriesOnRateLimit(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit reached"}}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"second time lucky"}}],
			"usage":{"prompt_tokens":10,"completion_tokens":5}
		}`))
	}))
	defer srv.Close()

	got, err := newTestClient(srv.URL).
		Complete(context.Background(), Request{Feature: "copy_gen", User: "hi"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "second time lucky" {
		t.Errorf("content = %q, want %q", got, "second time lucky")
	}
	if n := attempts.Load(); n != 2 {
		t.Errorf("provider attempts = %d, want 2", n)
	}
}

// A 400 is the provider telling us the request is wrong. Retrying it
// burns rate-limit budget for a call that cannot succeed.
func TestComplete_DoesNotRetryClientError(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"model not found"}}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).
		Complete(context.Background(), Request{Feature: "copy_gen", User: "hi"})
	if err == nil {
		t.Fatal("Complete: got nil error, want failure")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("error = %v, want it to carry the provider message", err)
	}
	if n := attempts.Load(); n != 1 {
		t.Errorf("provider attempts = %d, want 1 (no retry on 4xx)", n)
	}
}

func TestComplete_NoChoicesReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[],"usage":{}}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := newTestClient(srv.URL).Complete(ctx, Request{User: "hi"}); err == nil {
		t.Error("Complete: got nil error on an empty choices array, want failure")
	}
}

func TestCompleteJSON_UnwrapsFencedObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"Here you go:\n` +
			"```json\\n{\\\"scores\\\":[{\\\"variant\\\":1,\\\"accuracy\\\":0.9}]}\\n```" +
			`"}}],
			"usage":{"prompt_tokens":10,"completion_tokens":5}
		}`))
	}))
	defer srv.Close()

	got, err := newTestClient(srv.URL).
		CompleteJSON(context.Background(), Request{Feature: "copy_gen", User: "score these"})
	if err != nil {
		t.Fatalf("CompleteJSON: %v", err)
	}
	want := `{"scores":[{"variant":1,"accuracy":0.9}]}`
	if got != want {
		t.Errorf("CompleteJSON = %q, want %q", got, want)
	}
}

func TestCompleteJSON_NoObjectInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"I am unable to score these variants."}}],
			"usage":{}
		}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv.URL).
		CompleteJSON(context.Background(), Request{User: "score these"}); err == nil {
		t.Error("CompleteJSON: got nil error on a prose-only response, want failure")
	}
}

// ── small helpers ────────────────────────────────────────────

func TestEnvOr(t *testing.T) {
	t.Setenv("OWNSTALL_TEST_MODEL", "  configured-model  ")
	if got := envOr("OWNSTALL_TEST_MODEL", "fallback"); got != "configured-model" {
		t.Errorf("envOr = %q, want %q", got, "configured-model")
	}

	t.Setenv("OWNSTALL_TEST_MODEL", "   ")
	if got := envOr("OWNSTALL_TEST_MODEL", "fallback"); got != "fallback" {
		t.Errorf("envOr on a whitespace-only value = %q, want %q", got, "fallback")
	}

	if got := envOr("OWNSTALL_TEST_UNSET", "fallback"); got != "fallback" {
		t.Errorf("envOr on an unset key = %q, want %q", got, "fallback")
	}
}

// Empty identifiers must reach Postgres as NULL: "" is not a valid uuid
// and would fail the insert.
func TestNullableColumns(t *testing.T) {
	if got := nullableUUID(""); got != nil {
		t.Errorf("nullableUUID(\"\") = %v, want nil", got)
	}
	if got := nullableUUID("abc"); got != "abc" {
		t.Errorf("nullableUUID(%q) = %v, want %q", "abc", got, "abc")
	}
	if got := nullableText(""); got != nil {
		t.Errorf("nullableText(\"\") = %v, want nil", got)
	}
	if got := nullableText("ref"); got != "ref" {
		t.Errorf("nullableText(%q) = %v, want %q", "ref", got, "ref")
	}
}

func TestMath0(t *testing.T) {
	if got := math0(-1.5); got != 0 {
		t.Errorf("math0(-1.5) = %v, want 0", got)
	}
	if got := math0(2.5); got != 2.5 {
		t.Errorf("math0(2.5) = %v, want 2.5", got)
	}
}

func TestNewClient_DisabledWithoutKeys(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	c := NewClient(nil)
	if c == nil {
		t.Fatal("NewClient returned nil; it must never panic or return nil")
	}
	if c.Enabled() {
		t.Error("Enabled() = true with no provider keys set")
	}
	// The budget must still be sane so the breaker is not wedged open at $0.
	if c.costLimitUSD != defaultCostLimitUSD {
		t.Errorf("costLimitUSD = %v, want %v", c.costLimitUSD, defaultCostLimitUSD)
	}
}

func TestNewClient_MalformedCostLimitFallsBack(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key")
	t.Setenv("AI_MONTHLY_COST_LIMIT_USD", "not-a-number")

	if c := NewClient(nil); c.costLimitUSD != defaultCostLimitUSD {
		t.Errorf("costLimitUSD = %v, want the default %v", c.costLimitUSD, defaultCostLimitUSD)
	}

	// Zero would wedge the breaker permanently open.
	t.Setenv("AI_MONTHLY_COST_LIMIT_USD", "0")
	if c := NewClient(nil); c.costLimitUSD != defaultCostLimitUSD {
		t.Errorf("costLimitUSD = %v on a zero limit, want the default %v",
			c.costLimitUSD, defaultCostLimitUSD)
	}

	t.Setenv("AI_MONTHLY_COST_LIMIT_USD", "12.50")
	if c := NewClient(nil); c.costLimitUSD != 12.50 {
		t.Errorf("costLimitUSD = %v, want 12.50", c.costLimitUSD)
	}
}
