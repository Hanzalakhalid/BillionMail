-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION current_company_id() RETURNS UUID
LANGUAGE SQL STABLE AS $$
    SELECT NULLIF(current_setting('app.company_id', TRUE), '')::UUID
$$;

DO $$
DECLARE
    table_name TEXT;
    tenant_tables TEXT[] := ARRAY[
        'company_members', 'company_invitations', 'company_settings',
        'company_subscriptions', 'tenant_usage', 'outbox_events'
    ];
BEGIN
    FOREACH table_name IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', table_name);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON %I USING (company_id = current_company_id()) WITH CHECK (company_id = current_company_id())',
            table_name
        );
    END LOOP;
END $$;

ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;
CREATE POLICY audit_logs_tenant_isolation ON audit_logs
    USING (company_id IS NULL OR company_id = current_company_id())
    WITH CHECK (company_id IS NULL OR company_id = current_company_id());
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;
ALTER TABLE audit_logs DISABLE ROW LEVEL SECURITY;
DO $$
DECLARE
    table_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['company_members','company_invitations','company_settings','company_subscriptions','tenant_usage','outbox_events'] LOOP
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', table_name);
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', table_name);
    END LOOP;
END $$;
DROP FUNCTION IF EXISTS current_company_id();
-- +goose StatementEnd
