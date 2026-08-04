-- ─────────────────────────────────────────────────────────────
-- Migration 001: Core tables — tenants and users
-- ─────────────────────────────────────────────────────────────

-- EXTENSION: pgcrypto for UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─────────────────────────────────────────────────────────────
-- TENANTS
-- Each tenant is one store owner / business using Ownstall.
-- Every piece of data links back to a tenant.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tenants (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    subdomain   TEXT        NOT NULL UNIQUE,    -- slug for /:subdomain URL
    plan        TEXT        NOT NULL DEFAULT 'free' CHECK (plan IN ('free', 'pro', 'enterprise')),
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for subdomain lookups (used on every storefront page load)
CREATE INDEX idx_tenants_subdomain ON tenants(subdomain);

-- ─────────────────────────────────────────────────────────────
-- USERS
-- A user belongs to exactly one tenant.
-- In a future iteration, users could belong to multiple tenants.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email               TEXT        NOT NULL UNIQUE,
    password_hash       TEXT        NOT NULL,
    first_name          TEXT,
    last_name           TEXT,
    role                TEXT        NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'admin', 'staff')),
    is_active           BOOLEAN     NOT NULL DEFAULT true,
    email_verified_at   TIMESTAMPTZ,
    last_login_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for email lookups (used on every login)
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_tenant_id ON users(tenant_id);

-- ─────────────────────────────────────────────────────────────
-- ROW LEVEL SECURITY
-- This is the core of multi-tenant isolation.
-- It prevents one tenant's queries from seeing another tenant's data.
-- ─────────────────────────────────────────────────────────────

-- Enable RLS on both tables
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;

-- Tenants policy: a tenant can only see their own row
-- current_setting() reads the tenant_id we set at the start of each request
CREATE POLICY tenant_isolation ON tenants
    USING (id = current_setting('app.current_tenant_id', true)::uuid);

-- Users policy: users can only see users in their tenant
CREATE POLICY user_tenant_isolation ON users
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- ─────────────────────────────────────────────────────────────
-- REFRESH TOKENS
-- Stores JWT refresh tokens so we can invalidate them (logout).
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL UNIQUE,    -- Store hash, not raw token
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,                    -- NULL = still valid
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- ─────────────────────────────────────────────────────────────
-- UPDATED_AT TRIGGER
-- Automatically updates updated_at on any row change
-- ─────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();