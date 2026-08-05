-- ─────────────────────────────────────────────────────────────
-- Migration 005: Reply — Tickets, Messages, Assignments
-- ─────────────────────────────────────────────────────────────

-- ─────────────────────────────────────────────────────────────
-- TICKET LABELS
-- Custom labels merchants can create (e.g. "Billing", "Urgent")
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS ticket_labels (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID    NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    color       TEXT    NOT NULL DEFAULT '#6366f1',  -- Hex color for badge
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_ticket_labels_tenant ON ticket_labels(tenant_id);

-- ─────────────────────────────────────────────────────────────
-- TICKETS
-- Each ticket represents one customer support conversation.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tickets (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Ticket number (human readable, e.g. #1042)
    -- SERIAL increments globally across tenants (fine for demo)
    number          SERIAL,

    subject         TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'pending', 'resolved', 'closed')),
    priority        TEXT        NOT NULL DEFAULT 'normal'
                    CHECK (priority IN ('low', 'normal', 'high', 'urgent')),

    -- Customer info
    customer_email  TEXT        NOT NULL,
    customer_name   TEXT,
    customer_user_id UUID       REFERENCES users(id) ON DELETE SET NULL,

    -- Assignment
    assignee_id     UUID        REFERENCES users(id) ON DELETE SET NULL,

    -- Source of the ticket
    source          TEXT        NOT NULL DEFAULT 'web'
                    CHECK (source IN ('web', 'email', 'api', 'manual')),

    -- Context
    order_id        UUID        REFERENCES orders(id) ON DELETE SET NULL,

    -- SLA tracking
    -- first_response_at: when the first staff reply was sent
    first_response_at   TIMESTAMPTZ,
    -- sla_first_response: deadline for first response (tenant-configurable)
    sla_first_response_at TIMESTAMPTZ,
    -- resolved_at: when status changed to resolved
    resolved_at         TIMESTAMPTZ,
    -- sla_resolution: deadline for resolution
    sla_resolution_at   TIMESTAMPTZ,
    -- last_alerted_at: when the last SLA breach alert was sent
    -- (prevents the 5-minute SLA monitor from re-alerting every run)
    last_alerted_at     TIMESTAMPTZ,

    -- AI draft (populated on Day 8)
    ai_draft_body       TEXT,
    ai_draft_confidence NUMERIC(3,2),
    ai_draft_generated_at TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tickets_tenant         ON tickets(tenant_id);
CREATE INDEX idx_tickets_status         ON tickets(tenant_id, status);
CREATE INDEX idx_tickets_assignee       ON tickets(assignee_id);
CREATE INDEX idx_tickets_customer_email ON tickets(tenant_id, customer_email);
CREATE INDEX idx_tickets_priority       ON tickets(tenant_id, priority, status);

-- Ticket labels (many-to-many)
CREATE TABLE IF NOT EXISTS ticket_label_assignments (
    ticket_id   UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    label_id    UUID NOT NULL REFERENCES ticket_labels(id) ON DELETE CASCADE,
    PRIMARY KEY (ticket_id, label_id)
);

-- ─────────────────────────────────────────────────────────────
-- TICKET MESSAGES
-- Every reply, note, or automated message in a ticket thread.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS ticket_messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id       UUID        NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,

    -- Who sent the message
    author_type     TEXT        NOT NULL
                    CHECK (author_type IN ('customer', 'staff', 'system', 'ai')),
    author_id       UUID        REFERENCES users(id) ON DELETE SET NULL,
    author_email    TEXT,
    author_name     TEXT,

    -- Message content
    body            TEXT        NOT NULL,
    body_html       TEXT,                     -- HTML version if email-sourced

    -- Visibility
    is_internal     BOOLEAN     NOT NULL DEFAULT false, -- Internal notes not shown to customer

    -- Email metadata (when ticket came in via email)
    email_message_id TEXT,                    -- Email Message-ID header
    email_in_reply_to TEXT,                   -- References header

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_messages_ticket   ON ticket_messages(ticket_id);
CREATE INDEX idx_ticket_messages_email_id ON ticket_messages(email_message_id);

-- ─────────────────────────────────────────────────────────────
-- CANNED RESPONSES
-- Pre-written replies staff can insert with one click
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS canned_responses (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,          -- Internal name: "Refund Policy"
    shortcut    TEXT,                          -- Type /refund to insert
    body        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, shortcut)
);

CREATE INDEX idx_canned_responses_tenant ON canned_responses(tenant_id);

-- ─────────────────────────────────────────────────────────────
-- TICKET EVENTS (audit trail)
-- Records every status change, assignment, label addition, etc.
-- Shown in ticket timeline as activity entries.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS ticket_events (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID        NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id     UUID        REFERENCES users(id) ON DELETE SET NULL,
    event_type  TEXT        NOT NULL,
                -- 'created','status_changed','assigned','priority_changed',
                -- 'label_added','label_removed','note_added'
    metadata    JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_events_ticket ON ticket_events(ticket_id);

-- RLS
ALTER TABLE ticket_labels            ENABLE ROW LEVEL SECURITY;
ALTER TABLE ticket_label_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE tickets                  ENABLE ROW LEVEL SECURITY;
ALTER TABLE ticket_messages          ENABLE ROW LEVEL SECURITY;
ALTER TABLE canned_responses         ENABLE ROW LEVEL SECURITY;
ALTER TABLE ticket_events            ENABLE ROW LEVEL SECURITY;

CREATE POLICY tickets_isolation ON tickets
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY ticket_labels_isolation ON ticket_labels
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY ticket_label_assignments_isolation ON ticket_label_assignments
    USING (ticket_id IN (
        SELECT id FROM tickets
        WHERE tenant_id = current_setting('app.current_tenant_id', true)::uuid
    ));

CREATE POLICY ticket_messages_isolation ON ticket_messages
    USING (ticket_id IN (
        SELECT id FROM tickets
        WHERE tenant_id = current_setting('app.current_tenant_id', true)::uuid
    ));

CREATE POLICY canned_responses_isolation ON canned_responses
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY ticket_events_isolation ON ticket_events
    USING (ticket_id IN (
        SELECT id FROM tickets
        WHERE tenant_id = current_setting('app.current_tenant_id', true)::uuid
    ));

-- Updated_at triggers
CREATE TRIGGER tickets_updated_at
    BEFORE UPDATE ON tickets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Seed default canned responses
-- (Done programmatically per tenant in Go)
