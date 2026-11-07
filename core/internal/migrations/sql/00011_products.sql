-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS marketing_products (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 name VARCHAR(200) NOT NULL,
 sku VARCHAR(160) NOT NULL DEFAULT '',
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 current_version_id UUID,
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'user',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_at TIMESTAMPTZ,
 UNIQUE(company_id,id)
);
CREATE UNIQUE INDEX marketing_product_sku ON marketing_products(company_id,LOWER(sku)) WHERE sku<>'' AND deleted_at IS NULL;
CREATE INDEX marketing_products_library ON marketing_products(company_id,created_at DESC,id) WHERE deleted_at IS NULL;
CREATE TABLE IF NOT EXISTS marketing_product_versions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 product_id UUID NOT NULL,
 version BIGINT NOT NULL CHECK(version>0),
 snapshot JSONB NOT NULL,
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'user',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_at TIMESTAMPTZ,
 UNIQUE(company_id,id),
 UNIQUE(company_id,id,product_id),
 UNIQUE(product_id,version),
 FOREIGN KEY(company_id,product_id) REFERENCES marketing_products(company_id,id)
);
ALTER TABLE marketing_products ADD CONSTRAINT marketing_product_current_version
 FOREIGN KEY(company_id,current_version_id,id) REFERENCES marketing_product_versions(company_id,id,product_id);
CREATE TABLE IF NOT EXISTS marketing_product_media (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 product_version_id UUID NOT NULL,
 asset_version_id UUID NOT NULL,
 position INTEGER NOT NULL CHECK(position BETWEEN 0 AND 19),
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'user',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_at TIMESTAMPTZ,
 UNIQUE(product_version_id,asset_version_id),
 UNIQUE(product_version_id,position),
 FOREIGN KEY(company_id,product_version_id) REFERENCES marketing_product_versions(company_id,id),
 FOREIGN KEY(company_id,asset_version_id) REFERENCES marketing_asset_versions(company_id,id)
);
CREATE TABLE IF NOT EXISTS marketing_product_sources (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 product_version_id UUID NOT NULL,
 kind VARCHAR(24) NOT NULL CHECK(kind IN ('manual','url','catalog','commerce')),
 url TEXT,
 verified_at TIMESTAMPTZ,
 metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'user',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_at TIMESTAMPTZ,
 FOREIGN KEY(company_id,product_version_id) REFERENCES marketing_product_versions(company_id,id)
);
CREATE FUNCTION protect_product_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (to_jsonb(NEW)-'updated_at'-'deleted_at') IS DISTINCT FROM (to_jsonb(OLD)-'updated_at'-'deleted_at') THEN
  RAISE EXCEPTION 'product version content and provenance are immutable';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketing_product_versions_immutable BEFORE UPDATE ON marketing_product_versions FOR EACH ROW EXECUTE FUNCTION protect_product_version();
CREATE TRIGGER marketing_product_media_immutable BEFORE UPDATE ON marketing_product_media FOR EACH ROW EXECUTE FUNCTION protect_product_version();
CREATE TRIGGER marketing_product_sources_immutable BEFORE UPDATE ON marketing_product_sources FOR EACH ROW EXECUTE FUNCTION protect_product_version();
DO $$ DECLARE item TEXT; BEGIN
 FOREACH item IN ARRAY ARRAY['marketing_products','marketing_product_versions','marketing_product_media','marketing_product_sources'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',item);
  EXECUTE format('CREATE POLICY tenant_isolation ON %I USING(company_id=current_company_id()) WITH CHECK(company_id=current_company_id())',item);
 END LOOP;
END $$;
-- +goose StatementEnd
-- +goose Down
-- Retain product versions and source provenance used by marketing plans.
SELECT 1;
