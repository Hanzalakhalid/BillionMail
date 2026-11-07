-- +goose Up
-- +goose StatementBegin
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS dispatch_generation INTEGER NOT NULL DEFAULT 0 CHECK(dispatch_generation>=0);
CREATE INDEX IF NOT EXISTS outbox_events_company_dead_letter ON outbox_events(company_id,dead_lettered_at DESC,id)
    WHERE dead_lettered_at IS NOT NULL;
-- +goose StatementEnd
-- +goose Down
-- Replay generations must not be reset while tasks may still exist in Redis.
SELECT 1;
