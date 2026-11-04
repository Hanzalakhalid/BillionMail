-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS whatsapp_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    provider VARCHAR(24) NOT NULL DEFAULT 'meta',
    waba_id VARCHAR(128) NOT NULL,
    business_name VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'connected'
        CHECK (status IN ('connecting','connected','degraded','expired','disconnected')),
    scopes TEXT[] NOT NULL DEFAULT '{}',
    token_expires_at TIMESTAMPTZ,
    last_verified_at TIMESTAMPTZ,
    reconnect_required_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (provider, waba_id)
);
CREATE INDEX IF NOT EXISTS whatsapp_accounts_company_status
    ON whatsapp_accounts(company_id, status) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS whatsapp_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    whatsapp_account_id UUID NOT NULL REFERENCES whatsapp_accounts(id) ON DELETE CASCADE,
    credential_type VARCHAR(40) NOT NULL,
    ciphertext BYTEA NOT NULL,
    encrypted_data_key BYTEA NOT NULL,
    key_arn TEXT NOT NULL,
    credential_version INTEGER NOT NULL DEFAULT 1,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    UNIQUE (whatsapp_account_id, credential_type, credential_version)
);
CREATE INDEX IF NOT EXISTS whatsapp_credentials_company_account
    ON whatsapp_credentials(company_id, whatsapp_account_id) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS whatsapp_phone_numbers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    whatsapp_account_id UUID NOT NULL REFERENCES whatsapp_accounts(id) ON DELETE CASCADE,
    provider VARCHAR(24) NOT NULL DEFAULT 'meta',
    phone_number_id VARCHAR(128) NOT NULL,
    display_phone_number VARCHAR(40),
    verified_name VARCHAR(255),
    quality_rating VARCHAR(32),
    messaging_limit_tier VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'active'
        CHECK (status IN ('pending','active','restricted','disconnected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (provider, phone_number_id)
);
CREATE INDEX IF NOT EXISTS whatsapp_phone_numbers_company_status
    ON whatsapp_phone_numbers(company_id, status) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS whatsapp_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    whatsapp_account_id UUID NOT NULL REFERENCES whatsapp_accounts(id) ON DELETE CASCADE,
    provider_template_id VARCHAR(128),
    name VARCHAR(512) NOT NULL,
    language VARCHAR(24) NOT NULL,
    category VARCHAR(40),
    status VARCHAR(32) NOT NULL,
    components JSONB NOT NULL DEFAULT '[]'::JSONB,
    quality_score JSONB,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, whatsapp_account_id, name, language)
);

CREATE TABLE IF NOT EXISTS webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    provider VARCHAR(24) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,
    phone_number_id UUID NOT NULL REFERENCES whatsapp_phone_numbers(id) ON DELETE CASCADE,
    event_type VARCHAR(80) NOT NULL DEFAULT 'envelope',
    payload JSONB NOT NULL,
    signature VARCHAR(255),
    status VARCHAR(24) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','processing','processed','failed','dead_lettered')),
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_event_id)
);
CREATE INDEX IF NOT EXISTS webhook_events_company_received
    ON webhook_events(company_id, received_at DESC);
CREATE INDEX IF NOT EXISTS webhook_events_pending
    ON webhook_events(status, available_at, received_at) WHERE status IN ('pending','failed');
CREATE INDEX IF NOT EXISTS webhook_events_phone_received
    ON webhook_events(phone_number_id, received_at DESC);

DO $$
DECLARE table_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'whatsapp_accounts','whatsapp_credentials','whatsapp_phone_numbers',
        'whatsapp_templates','webhook_events'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', table_name);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON %I USING (company_id = current_company_id()) WITH CHECK (company_id = current_company_id())',
            table_name
        );
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS whatsapp_templates;
DROP TABLE IF EXISTS whatsapp_phone_numbers;
DROP TABLE IF EXISTS whatsapp_credentials;
DROP TABLE IF EXISTS whatsapp_accounts;
-- +goose StatementEnd
