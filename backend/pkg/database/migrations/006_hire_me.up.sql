-- ─────────────────────────────────────────────────────────────
-- Migration 006: Hire Me — Creator Profiles, Bookings, Reviews
-- ─────────────────────────────────────────────────────────────

-- ─────────────────────────────────────────────────────────────
-- CREATOR PROFILES
-- A creator profile belongs to a user inside a tenant.
-- It is publicly visible at /:subdomain/hire
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS creator_profiles (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id             UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Public display info
    display_name        TEXT        NOT NULL,
    tagline             TEXT,                       -- One-line description
    bio                 TEXT,                       -- Full bio
    avatar_url          TEXT,
    cover_image_url     TEXT,

    -- Skills (array of strings: ["Go", "React", "Graphic Design"])
    skills              TEXT[]      NOT NULL DEFAULT '{}',

    -- Availability and pricing
    is_available        BOOLEAN     NOT NULL DEFAULT true,
    hourly_rate         NUMERIC(10,2),              -- NULL = project-based only
    min_project_budget  NUMERIC(10,2),

    -- Response time: 'within_hour', 'within_day', 'within_week'
    response_time       TEXT        NOT NULL DEFAULT 'within_day',

    -- Stripe Connect account for receiving payments
    stripe_account_id   TEXT,                       -- Connected Stripe account
    stripe_onboarded    BOOLEAN     NOT NULL DEFAULT false,

    -- Stats (denormalized for performance)
    total_bookings      INT         NOT NULL DEFAULT 0,
    completed_bookings  INT         NOT NULL DEFAULT 0,
    avg_rating          NUMERIC(3,2),
    total_reviews       INT         NOT NULL DEFAULT 0,

    -- Visibility
    is_published        BOOLEAN     NOT NULL DEFAULT false,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(tenant_id, user_id)
);

CREATE INDEX idx_creator_profiles_tenant    ON creator_profiles(tenant_id);
CREATE INDEX idx_creator_profiles_available ON creator_profiles(tenant_id, is_available, is_published);

-- ─────────────────────────────────────────────────────────────
-- PORTFOLIO ITEMS
-- Work samples displayed on the creator's profile
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS portfolio_items (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id      UUID        NOT NULL REFERENCES creator_profiles(id) ON DELETE CASCADE,
    title           TEXT        NOT NULL,
    description     TEXT,
    image_url       TEXT        NOT NULL,
    project_url     TEXT,
    tags            TEXT[]      NOT NULL DEFAULT '{}',
    position        INT         NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_portfolio_items_profile ON portfolio_items(profile_id);

-- ─────────────────────────────────────────────────────────────
-- SERVICES
-- Named packages a creator offers (e.g. "Logo Design — $299")
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS creator_services (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id      UUID        NOT NULL REFERENCES creator_profiles(id) ON DELETE CASCADE,
    title           TEXT        NOT NULL,           -- "Brand Identity Package"
    description     TEXT,
    price           NUMERIC(10,2) NOT NULL,
    delivery_days   INT         NOT NULL DEFAULT 7, -- Expected delivery time
    revisions       INT         NOT NULL DEFAULT 2, -- Number of revisions included
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    position        INT         NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_creator_services_profile ON creator_services(profile_id);

-- ─────────────────────────────────────────────────────────────
-- BOOKINGS
-- A booking is a contract between client and creator.
-- Payment is held in escrow until creator delivers.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS bookings (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    profile_id          UUID        NOT NULL REFERENCES creator_profiles(id),
    service_id          UUID        REFERENCES creator_services(id),

    -- Parties
    client_user_id      UUID        REFERENCES users(id) ON DELETE SET NULL,
    client_email        TEXT        NOT NULL,
    client_name         TEXT,

    -- Project details
    title               TEXT        NOT NULL,       -- Client-provided project name
    description         TEXT        NOT NULL,       -- Client brief
    requirements        TEXT,                       -- Additional requirements
    delivery_date       DATE,                       -- Requested completion date

    -- Pricing
    agreed_price        NUMERIC(10,2) NOT NULL,
    platform_fee        NUMERIC(10,2) NOT NULL DEFAULT 0,
    creator_payout      NUMERIC(10,2) NOT NULL DEFAULT 0,

    -- Status flow:
    -- pending → accepted → in_progress → delivered → completed
    -- pending → declined
    -- accepted/in_progress → cancelled
    status              TEXT        NOT NULL DEFAULT 'pending'
                        CHECK (status IN (
                            'pending', 'accepted', 'declined',
                            'in_progress', 'delivered',
                            'completed', 'cancelled', 'disputed'
                        )),

    -- Stripe payment
    stripe_payment_intent_id    TEXT,
    stripe_transfer_id          TEXT,               -- Set when payout released to creator
    payment_status              TEXT NOT NULL DEFAULT 'unpaid'
                                CHECK (payment_status IN (
                                    'unpaid', 'held', 'released', 'refunded'
                                )),

    -- Timestamps
    accepted_at         TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    declined_at         TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bookings_tenant      ON bookings(tenant_id);
CREATE INDEX idx_bookings_profile     ON bookings(profile_id);
CREATE INDEX idx_bookings_client      ON bookings(client_email);
CREATE INDEX idx_bookings_status      ON bookings(tenant_id, status);

-- ─────────────────────────────────────────────────────────────
-- BOOKING MESSAGES
-- In-app messaging between client and creator per booking
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS booking_messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id      UUID        NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    sender_user_id  UUID        REFERENCES users(id) ON DELETE SET NULL,
    sender_email    TEXT        NOT NULL,
    sender_name     TEXT,
    body            TEXT        NOT NULL,
    attachment_url  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_booking_messages_booking ON booking_messages(booking_id);

-- ─────────────────────────────────────────────────────────────
-- REVIEWS
-- Left by the client after a booking is completed.
-- One review per booking.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS reviews (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id      UUID        NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    profile_id      UUID        NOT NULL REFERENCES creator_profiles(id) ON DELETE CASCADE,
    reviewer_email  TEXT        NOT NULL,
    reviewer_name   TEXT,
    rating          INT         NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body            TEXT,
    is_published    BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(booking_id)
);

CREATE INDEX idx_reviews_profile ON reviews(profile_id);

-- RLS policies
ALTER TABLE creator_profiles   ENABLE ROW LEVEL SECURITY;
ALTER TABLE portfolio_items    ENABLE ROW LEVEL SECURITY;
ALTER TABLE creator_services   ENABLE ROW LEVEL SECURITY;
ALTER TABLE bookings           ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_messages   ENABLE ROW LEVEL SECURITY;
ALTER TABLE reviews            ENABLE ROW LEVEL SECURITY;

CREATE POLICY creator_profiles_isolation ON creator_profiles
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY portfolio_items_isolation ON portfolio_items
    USING (profile_id IN (
        SELECT id FROM creator_profiles
        WHERE tenant_id = current_setting('app.current_tenant_id', true)::uuid
    ));

CREATE POLICY creator_services_isolation ON creator_services
    USING (profile_id IN (
        SELECT id FROM creator_profiles
        WHERE tenant_id = current_setting('app.current_tenant_id', true)::uuid
    ));

CREATE POLICY bookings_isolation ON bookings
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY booking_messages_isolation ON booking_messages
    USING (booking_id IN (
        SELECT id FROM bookings
        WHERE tenant_id = current_setting('app.current_tenant_id', true)::uuid
    ));

CREATE POLICY reviews_isolation ON reviews
    USING (profile_id IN (
        SELECT id FROM creator_profiles
        WHERE tenant_id = current_setting('app.current_tenant_id', true)::uuid
    ));

-- Updated_at triggers
CREATE TRIGGER creator_profiles_updated_at
    BEFORE UPDATE ON creator_profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER bookings_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
