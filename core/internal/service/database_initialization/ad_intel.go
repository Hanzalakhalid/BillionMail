package database_initialization

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
)

func init() {
	registerHandler(func() {
		sqlList := []string{
			`-- Businesses found running ads in the Meta Ad Library
			CREATE TABLE IF NOT EXISTS bm_ad_advertisers (
				page_id VARCHAR(64) PRIMARY KEY,
				page_name VARCHAR(255) NOT NULL DEFAULT '',
				page_url TEXT NOT NULL DEFAULT '',
				website TEXT NOT NULL DEFAULT '',
				category VARCHAR(255) NOT NULL DEFAULT '',
				ad_topic TEXT NOT NULL DEFAULT '',
				ad_text TEXT NOT NULL DEFAULT '',
				cta VARCHAR(64) NOT NULL DEFAULT '',
				ad_count INTEGER NOT NULL DEFAULT 0,
				days_running INTEGER NOT NULL DEFAULT 0,
				platforms VARCHAR(255) NOT NULL DEFAULT '',
				likes INTEGER NOT NULL DEFAULT 0,
				score INTEGER NOT NULL DEFAULT 0,
				reasons TEXT NOT NULL DEFAULT '',
				keyword VARCHAR(255) NOT NULL DEFAULT '',
				country VARCHAR(8) NOT NULL DEFAULT '',
				lead_email VARCHAR(320) NOT NULL DEFAULT '', -- set once added to leads
				lead_group_id INTEGER NOT NULL DEFAULT 0,
				create_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
				update_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())
			)`,
			`CREATE INDEX IF NOT EXISTS idx_ad_advertisers_score ON bm_ad_advertisers(score);`,
			`CREATE INDEX IF NOT EXISTS idx_ad_advertisers_keyword ON bm_ad_advertisers(keyword);`,
		}

		for _, sql := range sqlList {
			_, err := g.DB().Exec(context.Background(), sql)
			if err != nil {
				g.Log().Error(context.Background(), "Failed to create ad advertisers table:", err)
				return
			}
		}
	})
}
