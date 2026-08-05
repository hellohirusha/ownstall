-- ─────────────────────────────────────────────────────────────
-- Migration 008: AI Features
-- A/B tests for generated copy, product embeddings for similarity
-- search, an audit trail of every model call, and image QA results.
-- ─────────────────────────────────────────────────────────────

-- ─────────────────────────────────────────────────────────────
-- A/B TEST EVENTS
-- Impressions and conversions per copy variant, so "the AI copy
-- converts better" is a measurement rather than a claim.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS ab_test_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    product_id      UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant         TEXT        NOT NULL,   -- 'professional', 'casual', 'punchy', 'original'
    session_id      TEXT        NOT NULL,
    event_type      TEXT        NOT NULL
                    CHECK (event_type IN ('impression', 'cart_add', 'purchase')),
    converted_at    TIMESTAMPTZ,            -- set when this impression later converts
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ab_events_product ON ab_test_events(product_id, variant);
CREATE INDEX idx_ab_events_session ON ab_test_events(session_id);
CREATE INDEX idx_ab_events_tenant  ON ab_test_events(tenant_id, created_at);

-- ─────────────────────────────────────────────────────────────
-- PRODUCT EMBEDDINGS
-- Vector representation of each product for similarity search.
--
-- 256 dimensions, not 768: vectors are produced locally by the
-- lexical-hash embedder (pkg/ai/embed.go) rather than a hosted
-- embedding API, and 256 hashed dimensions comfortably cover a
-- creator-sized catalogue. Swapping in a hosted model later means a
-- new migration for the new width plus a re-index — the `model`
-- column records which embedder produced each row.
-- ─────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS product_embeddings (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    embedding   vector(256) NOT NULL,
    source_text TEXT        NOT NULL,       -- what was embedded, for debugging
    model       TEXT        NOT NULL DEFAULT 'lexical-hash-v1',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(product_id)
);

-- HNSW rather than IVFFlat: an IVFFlat index trains its lists on the
-- rows present when it is built, so one created on an empty table
-- (which this is) gives poor recall until it is rebuilt. HNSW builds
-- incrementally and needs no training pass.
CREATE INDEX idx_product_embeddings_vector
    ON product_embeddings
    USING hnsw (embedding vector_cosine_ops);

CREATE INDEX idx_product_embeddings_tenant ON product_embeddings(tenant_id);

-- ─────────────────────────────────────────────────────────────
-- AI LOGS
-- One row per model call. Drives cost tracking (the circuit breaker
-- reloads month-to-date spend from here on boot), the AI dashboard,
-- and the evaluation report.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS ai_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Nullable: system-level calls (evaluation runs, maintenance) are
    -- not attributable to a tenant.
    tenant_id       UUID        REFERENCES tenants(id) ON DELETE SET NULL,
    feature         TEXT        NOT NULL,   -- 'copy_gen', 'recommendations', 'auto_reply', 'image_qa'
    model           TEXT        NOT NULL,
    input_tokens    INT         NOT NULL DEFAULT 0,
    output_tokens   INT         NOT NULL DEFAULT 0,
    cost_usd        NUMERIC(10,6) NOT NULL DEFAULT 0,
    latency_ms      INT         NOT NULL DEFAULT 0,
    success         BOOLEAN     NOT NULL DEFAULT true,
    error_message   TEXT,
    reference_id    TEXT,                   -- product_id, ticket_id, ...
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_logs_tenant  ON ai_logs(tenant_id, created_at);
CREATE INDEX idx_ai_logs_feature ON ai_logs(feature, created_at);
CREATE INDEX idx_ai_logs_created ON ai_logs(created_at);

-- ─────────────────────────────────────────────────────────────
-- IMAGE QA RESULTS
-- Vision-model verdicts on product photos, so listings can be gated
-- on image quality before they go live.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS image_qa_results (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    product_id      UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    image_url       TEXT        NOT NULL,
    quality_score   NUMERIC(3,2) NOT NULL,
    issues          TEXT[]      NOT NULL DEFAULT '{}',
    suggestion      TEXT,
    passed          BOOLEAN     NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Re-checking an image replaces its verdict instead of appending
    UNIQUE(product_id, image_url)
);

CREATE INDEX idx_image_qa_product ON image_qa_results(product_id);
CREATE INDEX idx_image_qa_tenant  ON image_qa_results(tenant_id);

-- RLS
ALTER TABLE ab_test_events     ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_embeddings ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_logs            ENABLE ROW LEVEL SECURITY;
ALTER TABLE image_qa_results   ENABLE ROW LEVEL SECURITY;

CREATE POLICY ab_test_events_isolation ON ab_test_events
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY product_embeddings_isolation ON product_embeddings
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY image_qa_results_isolation ON image_qa_results
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- ai_logs additionally admits rows with no tenant, which is how
-- system-level calls are recorded.
CREATE POLICY ai_logs_isolation ON ai_logs
    USING (
        tenant_id IS NULL
        OR tenant_id = current_setting('app.current_tenant_id', true)::uuid
    );
