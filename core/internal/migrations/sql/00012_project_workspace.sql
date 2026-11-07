-- +goose Up
-- +goose StatementBegin
ALTER TABLE marketing_projects ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0);
ALTER TABLE marketing_projects ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE marketing_projects ADD COLUMN source VARCHAR(80) NOT NULL DEFAULT 'user';
ALTER TABLE marketing_project_messages ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE marketing_project_messages ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE marketing_project_messages ADD COLUMN source VARCHAR(80) NOT NULL DEFAULT 'user';
CREATE TABLE marketing_project_events (
 id BIGSERIAL PRIMARY KEY,
 company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 project_id UUID NOT NULL,
 type VARCHAR(80) NOT NULL,
 payload JSONB NOT NULL,
 created_by UUID REFERENCES users(id) ON DELETE SET NULL,
 source VARCHAR(80) NOT NULL DEFAULT 'user',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_at TIMESTAMPTZ,
 FOREIGN KEY(company_id,project_id) REFERENCES marketing_projects(company_id,id)
);
CREATE INDEX marketing_project_events_cursor ON marketing_project_events(company_id,project_id,id);
ALTER TABLE marketing_project_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON marketing_project_events
 USING(company_id=current_company_id()) WITH CHECK(company_id=current_company_id());
CREATE FUNCTION protect_project_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (to_jsonb(NEW)-'updated_at'-'deleted_at') IS DISTINCT FROM (to_jsonb(OLD)-'updated_at'-'deleted_at') THEN
  RAISE EXCEPTION 'project history is immutable';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketing_project_messages_immutable BEFORE UPDATE ON marketing_project_messages FOR EACH ROW EXECUTE FUNCTION protect_project_history();
CREATE TRIGGER marketing_project_events_immutable BEFORE UPDATE ON marketing_project_events FOR EACH ROW EXECUTE FUNCTION protect_project_history();
-- +goose StatementEnd
-- +goose Down
-- Preserve conversation history and durable reconnect cursors.
SELECT 1;
