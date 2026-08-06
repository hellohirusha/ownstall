-- ─────────────────────────────────────────────────────────────
-- Migration 010: Marketplace
--
-- Turns Ownstall from "a tenant runs a store and buys from it" into a
-- three-sided marketplace:
--   * tenants open a stall and submit it for review
--   * platform admins approve, reject, restrict or suspend stalls
--   * buyers browse every approved stall and check out as guest or account
-- ─────────────────────────────────────────────────────────────

-- ─────────────────────────────────────────────────────────────
-- TENANTS — marketplace listing fields + moderation state
-- ─────────────────────────────────────────────────────────────
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS status      TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS tagline     TEXT,
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS category    TEXT,
    ADD COLUMN IF NOT EXISTS location    TEXT,
    ADD COLUMN IF NOT EXISTS logo_url    TEXT,
    ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reviewed_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reviewed_by  UUID,
    ADD COLUMN IF NOT EXISTS review_note  TEXT,
    -- Restrictions are a lighter tool than suspension: a stall stays visible
    -- but loses a specific capability.
    ADD COLUMN IF NOT EXISTS can_publish_products BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS can_accept_orders    BOOLEAN NOT NULL DEFAULT true;

DO $$
BEGIN
    ALTER TABLE tenants ADD CONSTRAINT tenants_status_check
        CHECK (status IN ('pending', 'approved', 'rejected', 'suspended'));
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Every stall that existed before review was introduced is grandfathered in.
-- Without this the column default ('pending') would hide every live
-- storefront the moment this migration runs.
UPDATE tenants
   SET status = 'approved',
       submitted_at = COALESCE(submitted_at, created_at),
       reviewed_at = COALESCE(reviewed_at, NOW())
 WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
CREATE INDEX IF NOT EXISTS idx_tenants_category ON tenants(category);

-- ─────────────────────────────────────────────────────────────
-- PLATFORM ADMINS
-- Operators of the marketplace itself. Deliberately NOT rows in `users`:
-- that table is tenant-scoped (tenant_id is NOT NULL) and an operator
-- belongs to no tenant.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS platform_admins (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    name          TEXT,
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_platform_admins_email ON platform_admins(email);

-- ─────────────────────────────────────────────────────────────
-- BUYERS
-- Shoppers. Not tenant-scoped: one buyer shops across every stall.
-- Guest checkout writes no row here at all.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS buyers (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email             TEXT        NOT NULL UNIQUE,
    password_hash     TEXT        NOT NULL,
    first_name        TEXT,
    last_name         TEXT,
    is_active         BOOLEAN     NOT NULL DEFAULT true,
    email_verified_at TIMESTAMPTZ,
    last_login_at     TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_buyers_email ON buyers(email);

-- Link orders to a buyer account when one was signed in. Guest orders keep
-- buyer_id NULL and are identified only by customer_email.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS buyer_id UUID REFERENCES buyers(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_orders_buyer_id ON orders(buyer_id);
-- Guest order history is looked up by email, so that path needs an index too.
CREATE INDEX IF NOT EXISTS idx_orders_customer_email ON orders(customer_email);

-- ─────────────────────────────────────────────────────────────
-- REFRESH TOKENS — now issued to three kinds of subject
-- One nullable FK per audience with a check that exactly one is set, rather
-- than three near-identical tables.
-- ─────────────────────────────────────────────────────────────
ALTER TABLE refresh_tokens
    ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE refresh_tokens
    ADD COLUMN IF NOT EXISTS buyer_id UUID REFERENCES buyers(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS admin_id UUID REFERENCES platform_admins(id) ON DELETE CASCADE;

DO $$
BEGIN
    ALTER TABLE refresh_tokens ADD CONSTRAINT refresh_tokens_single_subject
        CHECK (
            (user_id  IS NOT NULL)::int +
            (buyer_id IS NOT NULL)::int +
            (admin_id IS NOT NULL)::int = 1
        );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_buyer_id ON refresh_tokens(buyer_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_admin_id ON refresh_tokens(admin_id);

-- ─────────────────────────────────────────────────────────────
-- TERMS ACCEPTANCES
-- One row per acceptance, versioned. Bumping the terms version means an
-- existing acceptance no longer matches, which is how a re-accept is
-- triggered rather than silently binding a seller to terms they never saw.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS terms_acceptances (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     UUID        REFERENCES users(id) ON DELETE SET NULL,
    version     TEXT        NOT NULL,
    ip_address  TEXT,
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_terms_acceptances_tenant ON terms_acceptances(tenant_id);

-- ─────────────────────────────────────────────────────────────
-- MODERATION EVENTS
-- An append-only record of every operator decision, so "why is this stall
-- suspended" always has an answer.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tenant_moderation_events (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    admin_id   UUID        REFERENCES platform_admins(id) ON DELETE SET NULL,
    action     TEXT        NOT NULL CHECK (action IN (
                    'submit', 'approve', 'reject', 'suspend',
                    'restore', 'restrict', 'unrestrict'
               )),
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_moderation_events_tenant
    ON tenant_moderation_events(tenant_id, created_at DESC);

-- ─────────────────────────────────────────────────────────────
-- TRIGGERS
-- ─────────────────────────────────────────────────────────────
DROP TRIGGER IF EXISTS platform_admins_updated_at ON platform_admins;
CREATE TRIGGER platform_admins_updated_at
    BEFORE UPDATE ON platform_admins
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

DROP TRIGGER IF EXISTS buyers_updated_at ON buyers;
CREATE TRIGGER buyers_updated_at
    BEFORE UPDATE ON buyers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
