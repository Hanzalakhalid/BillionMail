package database_initialization

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

func init() {
	registerHandler(func() {
		sqlList := []string{
			`-- Automation workflows: a trigger plus an ordered list of steps
			CREATE TABLE IF NOT EXISTS bm_workflows (
				id SERIAL PRIMARY KEY,
				name VARCHAR(200) NOT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'draft', -- draft, active, paused
				trigger_type VARCHAR(32) NOT NULL,
				trigger_config JSONB NOT NULL DEFAULT '{}',
				steps JSONB NOT NULL DEFAULT '[]',
				enroll_since INTEGER NOT NULL DEFAULT 0, -- only contacts that entered the trigger after this time
				create_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
				update_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())
			)`,
			`-- One contact going through one workflow
			CREATE TABLE IF NOT EXISTS bm_workflow_runs (
				id BIGSERIAL PRIMARY KEY,
				workflow_id INTEGER NOT NULL REFERENCES bm_workflows(id) ON DELETE CASCADE,
				contact_id INTEGER NOT NULL,
				email VARCHAR(320) NOT NULL DEFAULT '',
				trigger_key VARCHAR(64) NOT NULL DEFAULT '',
				step_index INTEGER NOT NULL DEFAULT 0,
				status VARCHAR(16) NOT NULL DEFAULT 'active', -- active, completed, exited
				next_run_at INTEGER NOT NULL DEFAULT 0,
				last_error TEXT NOT NULL DEFAULT '',
				create_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
				update_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
				UNIQUE (workflow_id, contact_id, trigger_key)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_runs_due ON bm_workflow_runs(status, next_run_at)`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow ON bm_workflow_runs(workflow_id, status)`,
			`-- What each step did for each contact
			CREATE TABLE IF NOT EXISTS bm_workflow_logs (
				id BIGSERIAL PRIMARY KEY,
				workflow_id INTEGER NOT NULL REFERENCES bm_workflows(id) ON DELETE CASCADE,
				run_id BIGINT NOT NULL DEFAULT 0,
				contact_id INTEGER NOT NULL DEFAULT 0,
				email VARCHAR(320) NOT NULL DEFAULT '',
				step_index INTEGER NOT NULL DEFAULT 0,
				step_type VARCHAR(32) NOT NULL DEFAULT '',
				status VARCHAR(16) NOT NULL DEFAULT '', -- done, skipped, failed, exited
				detail TEXT NOT NULL DEFAULT '',
				create_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())
			)`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_logs_workflow ON bm_workflow_logs(workflow_id, create_time)`,
			// v2: branches, settings, per-message tracking
			`ALTER TABLE bm_workflows ADD COLUMN IF NOT EXISTS settings JSONB NOT NULL DEFAULT '{}'`,
			`ALTER TABLE bm_workflow_runs ADD COLUMN IF NOT EXISTS step_path VARCHAR(64) NOT NULL DEFAULT ''`,
			`ALTER TABLE bm_workflow_logs ADD COLUMN IF NOT EXISTS step_path VARCHAR(64) NOT NULL DEFAULT ''`,
			`ALTER TABLE bm_workflow_logs ADD COLUMN IF NOT EXISTS message_id VARCHAR(255) NOT NULL DEFAULT ''`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_logs_email_time ON bm_workflow_logs(email, create_time)`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_logs_message_id ON bm_workflow_logs(message_id) WHERE message_id <> ''`,
			// v3: manual messages wait here for someone to send or discard them
			`CREATE TABLE IF NOT EXISTS bm_workflow_reviews (
				id BIGSERIAL PRIMARY KEY,
				workflow_id INTEGER NOT NULL REFERENCES bm_workflows(id) ON DELETE CASCADE,
				run_id BIGINT NOT NULL DEFAULT 0,
				contact_id INTEGER NOT NULL DEFAULT 0,
				email VARCHAR(320) NOT NULL DEFAULT '',
				step_path VARCHAR(64) NOT NULL DEFAULT '',
				step_type VARCHAR(32) NOT NULL DEFAULT '',
				status VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending, sent, failed, discarded
				detail TEXT NOT NULL DEFAULT '',
				create_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
				resolve_time INTEGER NOT NULL DEFAULT 0
			)`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_reviews_workflow ON bm_workflow_reviews(workflow_id, status)`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_runs_email ON bm_workflow_runs(workflow_id, email)`,
			// per-message report looks up opens/clicks by message id
			`CREATE INDEX IF NOT EXISTS idx_mailstat_opened_message_id ON mailstat_opened(message_id)`,
			`CREATE INDEX IF NOT EXISTS idx_mailstat_clicked_message_id ON mailstat_clicked(message_id)`,
		}

		for _, sql := range sqlList {
			_, err := g.DB().Exec(context.Background(), sql)
			if err != nil {
				// keep going: later statements (e.g. indexes on other tables) are independent
				g.Log().Error(context.Background(), "Failed to set up workflow tables:", err)
			}
		}
	})
}
