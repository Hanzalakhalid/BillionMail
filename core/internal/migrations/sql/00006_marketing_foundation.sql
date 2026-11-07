-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS integration_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    provider VARCHAR(80) NOT NULL,
    capability VARCHAR(80) NOT NULL,
    name VARCHAR(160) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'unverified' CHECK (status IN ('unverified','connected','degraded','disconnected')),
    configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_verified_at TIMESTAMPTZ,
    last_error TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(company_id, id)
);
CREATE UNIQUE INDEX IF NOT EXISTS integration_connections_active_name
    ON integration_connections(company_id, provider, capability, LOWER(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS integration_connections_company_status
    ON integration_connections(company_id, status) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS integration_credential_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    connection_id UUID NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    ciphertext BYTEA NOT NULL,
    encrypted_data_key BYTEA NOT NULL,
    key_arn TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    UNIQUE(connection_id, version),
    FOREIGN KEY(company_id, connection_id) REFERENCES integration_connections(company_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS integration_credential_one_active
    ON integration_credential_versions(connection_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS integration_credential_company_connection
    ON integration_credential_versions(company_id, connection_id, version DESC);

CREATE TABLE IF NOT EXISTS marketing_business_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL UNIQUE REFERENCES companies(id) ON DELETE CASCADE,
    name VARCHAR(160) NOT NULL,
    website TEXT,
    industry VARCHAR(160),
    description TEXT,
    brand_voice TEXT,
    guidelines JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS marketing_projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    objective TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'discovery' CHECK (status IN ('discovery','planning','generating','review','approved','scheduled','running','paused','completed','rejected','cancelled','failed')),
    requirements JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(company_id, id)
);
CREATE INDEX IF NOT EXISTS marketing_projects_company_status
    ON marketing_projects(company_id, status, updated_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS marketing_project_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    project_id UUID NOT NULL,
    role VARCHAR(16) NOT NULL CHECK (role IN ('user','assistant','system','tool')),
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY(company_id, project_id) REFERENCES marketing_projects(company_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS marketing_project_messages_timeline
    ON marketing_project_messages(company_id, project_id, created_at, id);

CREATE TABLE IF NOT EXISTS marketing_strategy_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    project_id UUID NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    document JSONB NOT NULL,
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, version),
    UNIQUE(company_id, id),
    FOREIGN KEY(company_id, project_id) REFERENCES marketing_projects(company_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS marketing_approval_manifests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    project_id UUID NOT NULL,
    strategy_version_id UUID NOT NULL,
    scope VARCHAR(24) NOT NULL CHECK (scope IN ('strategy','execution','optimization')),
    manifest JSONB NOT NULL,
    manifest_hash CHAR(64) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'awaiting_approval' CHECK (status IN ('awaiting_approval','approved','rejected','superseded','revoked')),
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    decided_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at TIMESTAMPTZ,
    UNIQUE(project_id, manifest_hash),
    FOREIGN KEY(company_id, project_id) REFERENCES marketing_projects(company_id, id) ON DELETE CASCADE,
    FOREIGN KEY(company_id, strategy_version_id) REFERENCES marketing_strategy_versions(company_id, id)
);
CREATE INDEX IF NOT EXISTS marketing_approvals_company_pending
    ON marketing_approval_manifests(company_id, created_at DESC) WHERE status = 'awaiting_approval';

DO $$
DECLARE table_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'integration_connections','integration_credential_versions','marketing_business_profiles',
        'marketing_projects','marketing_project_messages','marketing_strategy_versions','marketing_approval_manifests'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', table_name);
        EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (company_id = current_company_id()) WITH CHECK (company_id = current_company_id())', table_name);
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Production marketing records and encrypted credentials are intentionally retained.
SELECT 1;
