-- ─────────────────────────────────────────────────────────────
-- Migration 009: Audit Logs
-- Who changed what, from where. Written for every GraphQL mutation.
-- ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_logs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Both nullable and ON DELETE SET NULL: an audit trail has to
    -- outlive the tenant or user it describes, otherwise deleting an
    -- account erases the record of what that account did.
    tenant_id   UUID        REFERENCES tenants(id) ON DELETE SET NULL,
    user_id     UUID        REFERENCES users(id) ON DELETE SET NULL,
    action      TEXT        NOT NULL,   -- GraphQL operation name
    ip_address  TEXT,
    user_agent  TEXT,
    success     BOOLEAN     NOT NULL DEFAULT true,
    metadata    JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_tenant  ON audit_logs(tenant_id, created_at DESC);
CREATE INDEX idx_audit_logs_user    ON audit_logs(user_id, created_at DESC);
CREATE INDEX idx_audit_logs_action  ON audit_logs(action, created_at DESC);
-- Supports "show me every failed write in the last hour" without a
-- scan of the whole table
CREATE INDEX idx_audit_logs_failures ON audit_logs(created_at DESC) WHERE NOT success;

ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

-- Unauthenticated mutations (login, signup, public storefront calls)
-- have no tenant, so those rows must remain insertable and readable
-- by the system path.
CREATE POLICY audit_logs_isolation ON audit_logs
    USING (
        tenant_id IS NULL
        OR tenant_id = current_setting('app.current_tenant_id', true)::uuid
    );
