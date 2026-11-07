-- +goose Up
-- +goose StatementBegin
ALTER TABLE marketing_business_profiles
 ADD COLUMN IF NOT EXISTS locale VARCHAR(32) NOT NULL DEFAULT 'en',
 ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
 ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
 ADD COLUMN IF NOT EXISTS source VARCHAR(80) NOT NULL DEFAULT 'user';
CREATE TABLE IF NOT EXISTS marketing_mutation_receipts (
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 operation VARCHAR(100) NOT NULL,
 idempotency_key VARCHAR(128) NOT NULL,
 request_hash CHAR(64) NOT NULL,
 response JSONB NOT NULL,
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'api',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(company_id,operation,idempotency_key)
);
ALTER TABLE marketing_mutation_receipts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON marketing_mutation_receipts
 USING(company_id=current_company_id()) WITH CHECK(company_id=current_company_id());
-- +goose StatementEnd
-- +goose Down
-- Preserve business context and mutation audit receipts.
SELECT 1;
