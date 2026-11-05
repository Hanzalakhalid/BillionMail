-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chatbot_flows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    phone_number_id UUID NOT NULL REFERENCES whatsapp_phone_numbers(id) ON DELETE CASCADE,
    name VARCHAR(160) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','active','paused')),
    active_version_id UUID,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS chatbot_flows_company_status
    ON chatbot_flows(company_id, status, updated_at DESC) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS chatbot_flows_one_active_per_number
    ON chatbot_flows(company_id, phone_number_id) WHERE status = 'active' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS chatbot_flow_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    flow_id UUID NOT NULL REFERENCES chatbot_flows(id) ON DELETE CASCADE,
    version INTEGER NOT NULL CHECK (version > 0),
    status VARCHAR(24) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','published','archived')),
    graph JSONB NOT NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    UNIQUE (flow_id, version)
);
CREATE INDEX IF NOT EXISTS chatbot_flow_versions_company_flow
    ON chatbot_flow_versions(company_id, flow_id, version DESC);
CREATE UNIQUE INDEX IF NOT EXISTS chatbot_flow_versions_one_draft
    ON chatbot_flow_versions(flow_id) WHERE status = 'draft';
ALTER TABLE chatbot_flows
    ADD CONSTRAINT chatbot_flows_active_version_fk
    FOREIGN KEY (active_version_id) REFERENCES chatbot_flow_versions(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS whatsapp_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    phone_number_id UUID NOT NULL REFERENCES whatsapp_phone_numbers(id) ON DELETE CASCADE,
    wa_id VARCHAR(64) NOT NULL,
    profile_name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, phone_number_id, wa_id)
);

CREATE TABLE IF NOT EXISTS whatsapp_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    phone_number_id UUID NOT NULL REFERENCES whatsapp_phone_numbers(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL REFERENCES whatsapp_contacts(id) ON DELETE CASCADE,
    status VARCHAR(24) NOT NULL DEFAULT 'open' CHECK (status IN ('open','pending','closed')),
    automation_mode VARCHAR(24) NOT NULL DEFAULT 'bot' CHECK (automation_mode IN ('bot','human','paused')),
    last_customer_message_at TIMESTAMPTZ,
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, phone_number_id, contact_id)
);
CREATE INDEX IF NOT EXISTS whatsapp_conversations_inbox
    ON whatsapp_conversations(company_id, status, last_message_at DESC);

CREATE TABLE IF NOT EXISTS whatsapp_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES whatsapp_conversations(id) ON DELETE CASCADE,
    provider_message_id VARCHAR(255),
    direction VARCHAR(16) NOT NULL CHECK (direction IN ('inbound','outbound')),
    message_type VARCHAR(32) NOT NULL DEFAULT 'text',
    body TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(24) NOT NULL CHECK (status IN ('received','queued','sent','delivered','read','failed')),
    error_code VARCHAR(80),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS whatsapp_messages_provider_unique
    ON whatsapp_messages(company_id, provider_message_id) WHERE provider_message_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS whatsapp_messages_conversation_time
    ON whatsapp_messages(company_id, conversation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS chatbot_conversation_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL UNIQUE REFERENCES whatsapp_conversations(id) ON DELETE CASCADE,
    flow_id UUID NOT NULL REFERENCES chatbot_flows(id) ON DELETE CASCADE,
    flow_version_id UUID NOT NULL REFERENCES chatbot_flow_versions(id) ON DELETE RESTRICT,
    current_node_id VARCHAR(100),
    status VARCHAR(24) NOT NULL DEFAULT 'waiting' CHECK (status IN ('running','waiting','completed','failed','paused')),
    variables JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS chatbot_states_company_flow
    ON chatbot_conversation_states(company_id, flow_id, status, updated_at DESC);

DO $$
DECLARE table_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'chatbot_flows','chatbot_flow_versions','whatsapp_contacts',
        'whatsapp_conversations','whatsapp_messages','chatbot_conversation_states'
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
DROP TABLE IF EXISTS chatbot_conversation_states;
DROP TABLE IF EXISTS whatsapp_messages;
DROP TABLE IF EXISTS whatsapp_conversations;
DROP TABLE IF EXISTS whatsapp_contacts;
ALTER TABLE chatbot_flows DROP CONSTRAINT IF EXISTS chatbot_flows_active_version_fk;
DROP TABLE IF EXISTS chatbot_flow_versions;
DROP TABLE IF EXISTS chatbot_flows;
-- +goose StatementEnd
