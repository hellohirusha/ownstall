# ADR-004: Groq as the AI provider, behind a metered client

**Status**: Accepted
**Date**: 2026-08-05

## Context

Four features call a language model: product copy generation, product
recommendations, support auto-reply, and image quality assessment. Together
they are the most expensive and least predictable dependency in the system —
a retry loop against a paid API is the one bug in this codebase that can
generate a bill while nobody is watching.

## Decision

Groq as the default provider, OpenAI as a drop-in fallback, both behind one
client in `pkg/ai` that meters every call.

Provider selection is by environment variable. Groq's API is
OpenAI-compatible, so the two differ only in base URL, key and model name:

```go
if key := os.Getenv("GROQ_API_KEY"); key != "" {
    c.apiKey, c.baseURL = key, groqBaseURL
    c.model = envOr("GROQ_MODEL", defaultGroqModel)
} else if key := os.Getenv("OPENAI_API_KEY"); key != "" {
    c.apiKey, c.baseURL = key, openAIBaseURL
    c.model = envOr("OPENAI_MODEL", defaultOpenAIModel)
}
```

Every call is priced from its token usage, accumulated against a monthly
budget, and written to `ai_logs`. When month-to-date spend reaches
`AI_MONTHLY_COST_LIMIT_USD` the circuit breaker opens and calls fail fast.

## Rationale

**Cost control before provider choice.** The breaker matters more than which
model answers. Spend is accumulated in micro-dollars in an atomic integer,
and reseeded from `ai_logs` on the first call of each month — so the breaker
survives the restarts Railway performs on every deploy, and resets at the
month boundary without a scheduled job.

Unrecognised models are priced from a deliberately expensive fallback table,
so an unknown model trips the breaker early rather than late.

**Groq for development** because its free tier covers the whole build and
demo, and its inference is fast enough that copy generation feels
interactive. **OpenAI as fallback** because it is better on the harder
prompts and the swap is one environment variable.

**Degradation over failure.** With no key configured the client returns
`ErrDisabled` and AI features become no-ops. The API still boots and every
other feature works. An AI provider outage must not be a site outage.

## Consequences

Model identifiers rot. The `llama-3.1-70b` generation named in the original
build plan was decommissioned during the build; the default is now
`llama-3.3-70b-versatile`. Pinning a model in code means tracking a provider's
deprecation schedule, which is why the model is an environment variable with
a code default rather than a constant.

The free tier's rate limits are tight enough to shape the design. Copy
generation produces three variants and scores them; scoring them in one
comparative call rather than three separate ones halves the request count,
and rating variants side by side turned out to produce better spread than
rating each in isolation.

Reasoning models required special handling. The vision model Groq currently
serves emits a `<think>` trace before its answer, which would otherwise be
published verbatim as a product description and whose braces defeat JSON
extraction. The client strips it, and sets `reasoning_effort: none` where
the provider honours it — on the vision model that cuts a reply from roughly
900 completion tokens to 80, which is the difference between fitting the
per-minute token budget and exhausting it on one image.

Small models ignore "respond with JSON only" often enough that response
parsing cannot assume clean output. `ExtractJSON` pulls the first balanced
object out of whatever the model wrapped it in — markdown fences, a
preamble, or both. This is the single most common cause of parse failures
and is unit-tested against the shapes actually observed.

## Revisiting

Nothing above is specific to Groq except the default model name and the
pricing table. Moving to OpenAI in production is an environment variable;
moving to a provider with a different wire format would mean reimplementing
`post`, and nothing else.
