-- ─────────────────────────────────────────────────────────────
-- Migration 007: Manufacturing Integration
-- Models the physical production pipeline.
-- ─────────────────────────────────────────────────────────────

-- ─────────────────────────────────────────────────────────────
-- PRODUCTION QUEUE
-- Every paid order gets a production_queue entry.
-- Workers move it through the production stages.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS production_queue (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    order_id        UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,

    -- Production stages:
    -- queued → art_review → printing → cutting → quality_check → packaging → shipped
    status          TEXT        NOT NULL DEFAULT 'queued'
                    CHECK (status IN (
                        'queued',        -- Order received, waiting to start
                        'art_review',    -- Checking customer artwork files
                        'printing',      -- Printing in progress
                        'cutting',       -- Die-cutting or shape cutting
                        'quality_check', -- QA inspection
                        'packaging',     -- Packing for shipment
                        'shipped',       -- Handed to carrier
                        'on_hold',       -- Issue requires attention
                        'cancelled'
                    )),

    -- Priority (rush orders get higher priority)
    priority        INT         NOT NULL DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),

    -- Timestamps per stage
    queued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    art_review_at   TIMESTAMPTZ,
    printing_at     TIMESTAMPTZ,
    cutting_at      TIMESTAMPTZ,
    quality_check_at TIMESTAMPTZ,
    packaging_at    TIMESTAMPTZ,
    shipped_at      TIMESTAMPTZ,

    -- Production details
    machine_id      TEXT,               -- Which printer/cutter handled it
    operator_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    notes           TEXT,               -- Internal production notes

    -- Estimates
    estimated_completion_at TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(order_id)  -- One production record per order
);

CREATE INDEX idx_production_queue_tenant ON production_queue(tenant_id);
CREATE INDEX idx_production_queue_status ON production_queue(tenant_id, status);
CREATE INDEX idx_production_queue_order  ON production_queue(order_id);

-- ─────────────────────────────────────────────────────────────
-- PRODUCTION CAPACITY
-- Daily capacity planning. How many orders can ship per day?
-- Used for estimated completion time calculation.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS production_capacity (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID    NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    date            DATE    NOT NULL,
    max_orders      INT     NOT NULL DEFAULT 100,    -- Capacity for this day
    current_orders  INT     NOT NULL DEFAULT 0,      -- Orders scheduled this day
    is_holiday      BOOLEAN NOT NULL DEFAULT false,  -- No production on holidays
    UNIQUE(tenant_id, date)
);

CREATE INDEX idx_production_capacity_tenant ON production_capacity(tenant_id, date);

-- ─────────────────────────────────────────────────────────────
-- PRODUCTION EVENTS (audit trail)
-- Every status change in production is recorded here.
-- Used for throughput analytics and incident investigation.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS production_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    queue_id        UUID        NOT NULL REFERENCES production_queue(id) ON DELETE CASCADE,
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    from_status     TEXT,
    to_status       TEXT        NOT NULL,
    operator_id     UUID        REFERENCES users(id) ON DELETE SET NULL,
    machine_id      TEXT,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_production_events_queue  ON production_events(queue_id);
CREATE INDEX idx_production_events_tenant ON production_events(tenant_id, created_at);

-- ─────────────────────────────────────────────────────────────
-- WEBHOOK ENDPOINTS
-- Outbound webhooks fired when production status changes, so an
-- external manufacturing system can subscribe to the pipeline.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    url         TEXT        NOT NULL,
    event_types TEXT[]      NOT NULL DEFAULT '{}',
    secret      TEXT,       -- HMAC signature secret
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_endpoints_tenant ON webhook_endpoints(tenant_id, is_active);

-- ─────────────────────────────────────────────────────────────
-- DEVICE TOKENS
-- Expo push tokens registered by the mobile app, used to deliver
-- push notifications to a user's devices.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS device_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token       TEXT        NOT NULL UNIQUE,
    platform    TEXT        NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_device_tokens_user   ON device_tokens(user_id);
CREATE INDEX idx_device_tokens_tenant ON device_tokens(tenant_id);

-- RLS
ALTER TABLE production_queue     ENABLE ROW LEVEL SECURITY;
ALTER TABLE production_capacity  ENABLE ROW LEVEL SECURITY;
ALTER TABLE production_events    ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_endpoints    ENABLE ROW LEVEL SECURITY;
ALTER TABLE device_tokens        ENABLE ROW LEVEL SECURITY;

CREATE POLICY production_queue_isolation ON production_queue
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY production_capacity_isolation ON production_capacity
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY production_events_isolation ON production_events
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY webhook_endpoints_isolation ON webhook_endpoints
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY device_tokens_isolation ON device_tokens
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- Triggers
CREATE TRIGGER production_queue_updated_at
    BEFORE UPDATE ON production_queue
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER device_tokens_updated_at
    BEFORE UPDATE ON device_tokens
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
