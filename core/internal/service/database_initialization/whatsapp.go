package database_initialization

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
)

func init() {
	registerHandler(func() {
		sqlList := []string{
			`-- WhatsApp send logs
			CREATE TABLE IF NOT EXISTS bm_whatsapp_logs (
				id BIGSERIAL PRIMARY KEY,
				phone VARCHAR(32) NOT NULL,
				template VARCHAR(255) NOT NULL DEFAULT '',
				params TEXT NOT NULL DEFAULT '', -- template body values, joined with " | "
				message_id VARCHAR(255) NOT NULL DEFAULT '',
				status VARCHAR(16) NOT NULL DEFAULT 'pending', -- sent, failed
				error TEXT NOT NULL DEFAULT '',
				group_id INTEGER NOT NULL DEFAULT 0,
				create_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())
			)`,
			`CREATE INDEX IF NOT EXISTS idx_whatsapp_logs_create_time ON bm_whatsapp_logs(create_time);`,
			`CREATE INDEX IF NOT EXISTS idx_whatsapp_logs_status ON bm_whatsapp_logs(status);`,
		}

		for _, sql := range sqlList {
			_, err := g.DB().Exec(context.Background(), sql)
			if err != nil {
				g.Log().Error(context.Background(), "Failed to create whatsapp logs table:", err)
				return
			}
		}
	})
}
