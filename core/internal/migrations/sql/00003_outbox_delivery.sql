-- +goose Up
-- +goose StatementBegin
ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS dead_lettered_at TIMESTAMPTZ;

DROP INDEX IF EXISTS outbox_events_pending;
CREATE INDEX outbox_events_pending ON outbox_events (available_at, created_at)
    WHERE published_at IS NULL AND dead_lettered_at IS NULL;

CREATE TABLE IF NOT EXISTS outbox_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES outbox_events(id) ON DELETE CASCADE,
    consumer VARCHAR(120) NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    UNIQUE (event_id, consumer)
);
CREATE INDEX IF NOT EXISTS outbox_deliveries_company_time
    ON outbox_deliveries (company_id, completed_at DESC);

ALTER TABLE outbox_deliveries ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON outbox_deliveries;
CREATE POLICY tenant_isolation ON outbox_deliveries
    USING (company_id = current_company_id())
    WITH CHECK (company_id = current_company_id());
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON outbox_deliveries;
DROP TABLE IF EXISTS outbox_deliveries;
DROP INDEX IF EXISTS outbox_events_pending;
CREATE INDEX outbox_events_pending ON outbox_events (available_at, created_at)
    WHERE published_at IS NULL;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS dead_lettered_at;
-- +goose StatementEnd
