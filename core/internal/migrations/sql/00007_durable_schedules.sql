-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS marketing_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    name VARCHAR(160) NOT NULL,
    event_type VARCHAR(120) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    cron_expression VARCHAR(120),
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    next_run_at TIMESTAMPTZ,
    status VARCHAR(24) NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','completed','failed','cancelled')),
    leased_until TIMESTAMPTZ,
    leased_by VARCHAR(120),
    last_error TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    source VARCHAR(80) NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(company_id,id)
);
CREATE INDEX IF NOT EXISTS marketing_schedules_due ON marketing_schedules(next_run_at,id)
    WHERE status='active' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS marketing_job_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    schedule_id UUID NOT NULL,
    scheduled_for TIMESTAMPTZ NOT NULL,
    outbox_event_id UUID NOT NULL UNIQUE REFERENCES outbox_events(id),
    status VARCHAR(24) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','leased','running','retrying','succeeded','failed','dead-lettered','cancelled')),
    source VARCHAR(80) NOT NULL DEFAULT 'scheduler',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(schedule_id,scheduled_for),
    FOREIGN KEY(company_id,schedule_id) REFERENCES marketing_schedules(company_id,id)
);
CREATE INDEX IF NOT EXISTS marketing_job_runs_company_time ON marketing_job_runs(company_id,created_at DESC,id);

ALTER TABLE marketing_schedules ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON marketing_schedules
    USING(company_id=current_company_id()) WITH CHECK(company_id=current_company_id());
ALTER TABLE marketing_job_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON marketing_job_runs
    USING(company_id=current_company_id()) WITH CHECK(company_id=current_company_id());
-- +goose StatementEnd

-- +goose Down
-- Preserve execution history and pending work.
SELECT 1;
