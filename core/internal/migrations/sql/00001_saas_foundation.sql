-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legacy_account_id BIGINT UNIQUE REFERENCES account(account_id) ON DELETE SET NULL,
    username VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(320),
    password_hash VARCHAR(255) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'active' CHECK (status IN ('pending', 'active', 'suspended', 'deleted')),
    email_verified_at TIMESTAMPTZ,
    platform_super_admin BOOLEAN NOT NULL DEFAULT FALSE,
    membership_version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique ON users (LOWER(email)) WHERE email IS NOT NULL AND email <> '' AND deleted_at IS NULL;

INSERT INTO users (legacy_account_id, username, email, password_hash, status, email_verified_at, platform_super_admin)
SELECT account_id, username, NULLIF(LOWER(TRIM(email)), ''), password,
       CASE WHEN status = 1 THEN 'active' ELSE 'suspended' END,
       CASE WHEN NULLIF(TRIM(email), '') IS NULL THEN NULL ELSE NOW() END,
       EXISTS (
           SELECT 1 FROM account_role ar
           JOIN role r ON r.role_id = ar.role_id
           WHERE ar.account_id = account.account_id AND r.role_name = 'admin'
       )
FROM account
ON CONFLICT (legacy_account_id) DO UPDATE SET
    username = EXCLUDED.username,
    password_hash = EXCLUDED.password_hash,
    status = EXCLUDED.status,
    updated_at = NOW();

CREATE TABLE IF NOT EXISTS companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(160) NOT NULL,
    slug VARCHAR(80) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'active' CHECK (status IN ('trial', 'active', 'past_due', 'suspended', 'closed')),
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    locale VARCHAR(16) NOT NULL DEFAULT 'en',
    is_legacy_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS companies_slug_unique ON companies (LOWER(slug)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS companies_one_legacy_default ON companies (is_legacy_default) WHERE is_legacy_default;

INSERT INTO companies (name, slug, status, is_legacy_default, created_by)
SELECT 'BillionMail Workspace', 'billionmail', 'active', TRUE,
       (SELECT id FROM users ORDER BY platform_super_admin DESC, created_at ASC LIMIT 1)
WHERE NOT EXISTS (SELECT 1 FROM companies WHERE is_legacy_default);

CREATE TABLE IF NOT EXISTS company_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL CHECK (role IN ('owner', 'admin', 'manager', 'agent', 'viewer')),
    status VARCHAR(24) NOT NULL DEFAULT 'active' CHECK (status IN ('invited', 'active', 'suspended')),
    joined_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, user_id)
);
CREATE INDEX IF NOT EXISTS company_members_user_status ON company_members (user_id, status, company_id);

INSERT INTO company_members (company_id, user_id, role, status, joined_at)
SELECT c.id, u.id,
       CASE WHEN u.platform_super_admin THEN 'owner' ELSE 'admin' END,
       'active', NOW()
FROM companies c CROSS JOIN users u
WHERE c.is_legacy_default
ON CONFLICT (company_id, user_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS company_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    email VARCHAR(320) NOT NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('owner', 'admin', 'manager', 'agent', 'viewer')),
    token_hash BYTEA NOT NULL UNIQUE,
    invited_by UUID NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS company_invitations_pending_email ON company_invitations (company_id, LOWER(email)) WHERE accepted_at IS NULL AND revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_family UUID NOT NULL,
    refresh_token_hash BYTEA NOT NULL UNIQUE,
    active_company_id UUID REFERENCES companies(id) ON DELETE SET NULL,
    user_agent TEXT,
    ip_address INET,
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoke_reason VARCHAR(80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS auth_sessions_user_active ON auth_sessions (user_id, expires_at DESC) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS auth_sessions_family ON auth_sessions (token_family) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS company_settings (
    company_id UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO company_settings (company_id)
SELECT id FROM companies ON CONFLICT (company_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS subscription_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(40) NOT NULL UNIQUE,
    name VARCHAR(80) NOT NULL,
    entitlements JSONB NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO subscription_plans (code, name, entitlements) VALUES
('trial', 'Free Trial', '{"whatsapp_numbers":1,"team_members":3,"monthly_conversations":500,"monthly_messages":2000,"active_flows":3,"ai_tokens":0,"storage_bytes":1073741824}'::jsonb),
('basic', 'Basic', '{"whatsapp_numbers":1,"team_members":5,"monthly_conversations":2000,"monthly_messages":10000,"active_flows":10,"ai_tokens":100000,"storage_bytes":5368709120}'::jsonb),
('professional', 'Professional', '{"whatsapp_numbers":5,"team_members":25,"monthly_conversations":20000,"monthly_messages":100000,"active_flows":100,"ai_tokens":2000000,"storage_bytes":53687091200}'::jsonb),
('enterprise', 'Enterprise', '{"whatsapp_numbers":-1,"team_members":-1,"monthly_conversations":-1,"monthly_messages":-1,"active_flows":-1,"ai_tokens":-1,"storage_bytes":-1}'::jsonb)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS company_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL UNIQUE REFERENCES companies(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES subscription_plans(id),
    status VARCHAR(24) NOT NULL CHECK (status IN ('trialing', 'active', 'past_due', 'canceled', 'suspended')),
    provider VARCHAR(40),
    provider_customer_id VARCHAR(255),
    provider_subscription_id VARCHAR(255),
    trial_ends_at TIMESTAMPTZ,
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO company_subscriptions (company_id, plan_id, status, current_period_start, current_period_end)
SELECT c.id, p.id, 'active', DATE_TRUNC('month', NOW()), DATE_TRUNC('month', NOW()) + INTERVAL '1 month'
FROM companies c JOIN subscription_plans p ON p.code = 'enterprise'
WHERE c.is_legacy_default
ON CONFLICT (company_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS tenant_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    metric VARCHAR(80) NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    quantity BIGINT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, metric, period_start)
);
CREATE INDEX IF NOT EXISTS tenant_usage_period ON tenant_usage (company_id, period_start DESC, metric);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID REFERENCES companies(id) ON DELETE SET NULL,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(120) NOT NULL,
    resource_type VARCHAR(80) NOT NULL,
    resource_id VARCHAR(255),
    request_id VARCHAR(100),
    ip_address INET,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS audit_logs_company_time ON audit_logs (company_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_logs_actor_time ON audit_logs (actor_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID REFERENCES companies(id) ON DELETE CASCADE,
    aggregate_type VARCHAR(80) NOT NULL,
    aggregate_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(120) NOT NULL,
    payload JSONB NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    locked_by VARCHAR(120),
    published_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_type, idempotency_key)
);
CREATE INDEX IF NOT EXISTS outbox_events_pending ON outbox_events (available_at, created_at) WHERE published_at IS NULL;

-- Backfill the legacy tenant-owned data while keeping old code operational.
DO $$
DECLARE
    default_company UUID;
    table_name TEXT;
    tenant_tables TEXT[] := ARRAY[
        'bm_contact_groups', 'bm_contacts', 'email_templates', 'email_tasks',
        'recipient_info', 'bm_tags', 'bm_contact_tags', 'bm_whatsapp_logs',
        'bm_operation_logs', 'api_templates', 'api_mail_logs'
    ];
BEGIN
    SELECT id INTO default_company FROM companies WHERE is_legacy_default LIMIT 1;
    FOREACH table_name IN ARRAY tenant_tables LOOP
        IF to_regclass('public.' || table_name) IS NOT NULL THEN
            EXECUTE format('ALTER TABLE %I ADD COLUMN IF NOT EXISTS company_id UUID REFERENCES companies(id)', table_name);
            EXECUTE format('UPDATE %I SET company_id = $1 WHERE company_id IS NULL', table_name) USING default_company;
            EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (company_id)', 'idx_' || table_name || '_company_id', table_name);
        END IF;
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- The foundation migration is intentionally irreversible: it backfills identity
-- and tenant ownership into legacy production data.
SELECT 1;
