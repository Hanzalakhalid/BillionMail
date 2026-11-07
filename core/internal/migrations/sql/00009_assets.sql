-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS marketing_assets (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 title VARCHAR(200) NOT NULL,
 kind VARCHAR(24) NOT NULL CHECK(kind IN ('image','video','document')),
 current_version_id UUID,
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'upload',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_at TIMESTAMPTZ,
 UNIQUE(company_id,id)
);
CREATE TABLE IF NOT EXISTS marketing_asset_versions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 asset_id UUID NOT NULL,
 version INTEGER NOT NULL CHECK(version>0),
 status VARCHAR(24) NOT NULL DEFAULT 'awaiting_upload' CHECK(status IN ('awaiting_upload','processing','ready','rejected','failed')),
 declared_content_type VARCHAR(100) NOT NULL,
 declared_size BIGINT NOT NULL CHECK(declared_size BETWEEN 1 AND 20971520),
 quarantine_key TEXT NOT NULL,
 quarantine_cleaned_at TIMESTAMPTZ,
 object_key TEXT,
 sha256 CHAR(64),
 byte_size BIGINT,
 content_type VARCHAR(100),
 width INTEGER,
 height INTEGER,
 job_id UUID REFERENCES outbox_events(id),
 validation_owner UUID,
 validation_until TIMESTAMPTZ,
 upload_expires_at TIMESTAMPTZ NOT NULL,
 rejection_reason VARCHAR(200),
 provider_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'upload',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_at TIMESTAMPTZ,
 UNIQUE(company_id,id),
 UNIQUE(company_id,id,asset_id),
 UNIQUE(asset_id,version),
 FOREIGN KEY(company_id,asset_id) REFERENCES marketing_assets(company_id,id),
 CHECK(status<>'ready' OR (object_key IS NOT NULL AND sha256 IS NOT NULL AND byte_size>0 AND content_type IS NOT NULL))
);
ALTER TABLE marketing_assets ADD CONSTRAINT marketing_assets_current_version_fk
 FOREIGN KEY(company_id,current_version_id,id) REFERENCES marketing_asset_versions(company_id,id,asset_id);
CREATE TABLE IF NOT EXISTS marketing_asset_upload_requests (
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 idempotency_key VARCHAR(128) NOT NULL,
 request_hash CHAR(64) NOT NULL,
 version_id UUID NOT NULL,
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'api',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(company_id,idempotency_key),
 FOREIGN KEY(company_id,version_id) REFERENCES marketing_asset_versions(company_id,id)
);
CREATE INDEX IF NOT EXISTS marketing_assets_library ON marketing_assets(company_id,created_at DESC,id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS marketing_assets_validation ON marketing_asset_versions(company_id,status,validation_until);
CREATE FUNCTION protect_ready_asset_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.status='ready' AND (to_jsonb(NEW)-'updated_at'-'deleted_at'-'quarantine_cleaned_at') IS DISTINCT FROM (to_jsonb(OLD)-'updated_at'-'deleted_at'-'quarantine_cleaned_at') THEN
  RAISE EXCEPTION 'ready asset versions are immutable';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketing_asset_version_immutable BEFORE UPDATE ON marketing_asset_versions
 FOR EACH ROW EXECUTE FUNCTION protect_ready_asset_version();
DO $$ DECLARE item TEXT; BEGIN
 FOREACH item IN ARRAY ARRAY['marketing_assets','marketing_asset_versions','marketing_asset_upload_requests'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',item);
  EXECUTE format('CREATE POLICY tenant_isolation ON %I USING(company_id=current_company_id()) WITH CHECK(company_id=current_company_id())',item);
 END LOOP;
END $$;
CREATE UNIQUE INDEX marketing_asset_cleanup_schedule ON marketing_schedules(company_id)
 WHERE event_type='asset.quarantine.cleanup' AND source='system' AND deleted_at IS NULL;
INSERT INTO marketing_schedules(company_id,name,event_type,payload,cron_expression,timezone,next_run_at,source)
 SELECT id,'Expired upload cleanup','asset.quarantine.cleanup','{}'::jsonb,'0 * * * *',timezone,NOW()+INTERVAL '1 hour','system'
 FROM companies WHERE is_legacy_default=TRUE
 ON CONFLICT(company_id) WHERE event_type='asset.quarantine.cleanup' AND source='system' AND deleted_at IS NULL DO NOTHING;
-- +goose StatementEnd
-- +goose Down
-- Retain versioned asset provenance and upload history.
SELECT 1;
