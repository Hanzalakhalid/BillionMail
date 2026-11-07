package scheduler

import (
	"testing"
	"time"
)

func TestNextAcrossDaylightSaving(t *testing.T) {
	tests := []struct{ name, expression, zone, after, want string }{
		{"spring skips nonexistent hour", "30 2 * * *", "America/New_York", "2026-03-08T06:00:00Z", "2026-03-09T06:30:00Z"},
		{"fall first hour", "30 1 * * *", "America/New_York", "2026-11-01T04:00:00Z", "2026-11-01T05:30:00Z"},
		{"fall repeated hour", "30 1 * * *", "America/New_York", "2026-11-01T05:30:00Z", "2026-11-01T06:30:00Z"},
		{"workspace timezone", "0 9 * * *", "Asia/Karachi", "2026-11-07T00:00:00Z", "2026-11-07T04:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			after, _ := time.Parse(time.RFC3339, tt.after)
			got, err := Next(tt.expression, tt.zone, after)
			if err != nil || got.Format(time.RFC3339) != tt.want {
				t.Fatalf("got %s, %v; want %s", got, err, tt.want)
			}
		})
	}
}

func TestNextRejectsAmbiguousOrInvalidRecurrence(t *testing.T) {
	for _, tt := range []struct{ expression, zone string }{
		{"@every 1m", "UTC"}, {"CRON_TZ=UTC 0 9 * * *", "UTC"}, {"0 9 * * *", ""}, {"0 9 * * *", "Mars/Olympus"}, {"0 0 31 2 *", "UTC"},
	} {
		if _, err := Next(tt.expression, tt.zone, time.Now()); err == nil {
			t.Errorf("accepted %+v", tt)
		}
	}
}
