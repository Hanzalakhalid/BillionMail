package database_initialization

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
)

func init() {
	registerHandler(func() {
		sqlList := []string{
			`-- Video landing page activity (view, play, cta)
			CREATE TABLE IF NOT EXISTS bm_video_events (
				id BIGSERIAL PRIMARY KEY,
				contact_id INTEGER NOT NULL,
				event VARCHAR(16) NOT NULL,
				create_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())
			)`,
			`CREATE INDEX IF NOT EXISTS idx_video_events_contact ON bm_video_events(contact_id, event);`,
			`CREATE INDEX IF NOT EXISTS idx_video_events_create_time ON bm_video_events(create_time);`,
		}

		for _, sql := range sqlList {
			_, err := g.DB().Exec(context.Background(), sql)
			if err != nil {
				g.Log().Error(context.Background(), "Failed to create video events table:", err)
				return
			}
		}
	})
}
