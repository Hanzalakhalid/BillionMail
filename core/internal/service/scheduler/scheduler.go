// Package scheduler materializes due occurrences into the transactional outbox.
// It never calls a provider; delivery authorization remains a worker concern.
package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type Schedule struct {
	ID             uuid.UUID       `json:"id"`
	CompanyID      uuid.UUID       `json:"company_id"`
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	CronExpression string          `json:"cron_expression"`
	Timezone       string          `json:"timezone"`
	NextRunAt      time.Time       `json:"next_run_at"`
}

// Next uses an explicit IANA timezone and five-field cron. Embedded timezone
// overrides and interval descriptors are rejected to keep recurrence auditable.
func Next(expression, timezone string, after time.Time) (time.Time, error) {
	if strings.TrimSpace(timezone) == "" {
		return time.Time{}, errors.New("an IANA timezone is required")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid schedule timezone: %w", err)
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if strings.Contains(expression, "=") || strings.HasPrefix(strings.TrimSpace(expression), "@") {
		return time.Time{}, errors.New("use a five-field cron expression and separate timezone")
	}
	schedule, err := parser.Parse(expression)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression: %w", err)
	}
	next := schedule.Next(after.In(location))
	if next.IsZero() {
		return time.Time{}, errors.New("cron expression has no future occurrence")
	}
	return next.UTC(), nil
}

type Scheduler struct {
	WorkerID  string
	BatchSize int
	Lease     time.Duration
	// Only registered internal event types may be scheduled. Never accept event
	// names directly from agent output without capability validation.
	AllowedEvents map[string]bool
}

func (s Scheduler) Tick(ctx context.Context) (int, error) {
	if strings.TrimSpace(s.WorkerID) == "" || len(s.WorkerID) > 120 {
		return 0, errors.New("scheduler worker ID is required within 120 characters")
	}
	if s.BatchSize <= 0 || s.BatchSize > 500 {
		s.BatchSize = 50
	}
	if s.Lease < time.Second {
		s.Lease = time.Minute
	}
	var schedules []Schedule
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		rows, err := tx.Query(`WITH due AS (
   SELECT id FROM marketing_schedules WHERE status='active' AND deleted_at IS NULL
   AND next_run_at<=NOW() AND (leased_until IS NULL OR leased_until<=NOW())
   ORDER BY next_run_at,id FOR UPDATE SKIP LOCKED LIMIT ?
  ) UPDATE marketing_schedules s SET leased_by=?,leased_until=NOW()+(? * INTERVAL '1 second')
  FROM due WHERE s.id=due.id RETURNING s.id,s.company_id,s.event_type,s.payload,s.cron_expression,s.timezone,s.next_run_at`, s.BatchSize, s.WorkerID, int64(s.Lease.Seconds()))
		if err != nil {
			return err
		}
		return rows.Structs(&schedules)
	})
	if err != nil {
		return 0, err
	}
	count := 0
	var failures []error
	for _, schedule := range schedules {
		created, err := s.materialize(ctx, schedule)
		if err != nil {
			failures = append(failures, err)
		} else if created {
			count++
		}
	}
	return count, errors.Join(failures...)
}

func (s Scheduler) materialize(ctx context.Context, claimed Schedule) (bool, error) {
	created := false
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", claimed.CompanyID.String()); err != nil {
			return err
		}
		// Re-read under lock: a user may have paused or edited the schedule since
		// its lease was acquired. An expired or stolen lease cannot produce work.
		var current Schedule
		err := tx.Model("marketing_schedules").Where("id", claimed.ID).Where("company_id", claimed.CompanyID).
			Where("leased_by", s.WorkerID).Where("leased_until > NOW()").Where("next_run_at <= NOW()").Where("status", "active").WhereNull("deleted_at").LockUpdate().Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil || current.ID == uuid.Nil {
			return err
		}
		if !s.AllowedEvents[current.EventType] {
			_, err = tx.Model("marketing_schedules").Where("id", current.ID).Where("company_id", current.CompanyID).
				Data(g.Map{"status": "failed", "last_error": "event type is not registered for scheduling", "leased_by": nil, "leased_until": nil, "updated_at": time.Now().UTC()}).Update()
			return err
		}
		now := time.Now().UTC()
		var next any
		status := "completed"
		if current.CronExpression != "" {
			// Missed runs coalesce into one occurrence; recurrence resumes after now.
			nextTime, nextErr := Next(current.CronExpression, current.Timezone, now)
			if nextErr != nil {
				_, err = tx.Model("marketing_schedules").Where("id", current.ID).Where("company_id", current.CompanyID).
					Data(g.Map{"status": "failed", "last_error": nextErr.Error(), "leased_by": nil, "leased_until": nil, "updated_at": now}).Update()
				return err
			}
			next = nextTime
			status = "active"
		}
		eventID := uuid.New()
		key := "schedule:" + current.CompanyID.String() + ":" + current.ID.String() + ":" + current.NextRunAt.UTC().Format(time.RFC3339Nano)
		if _, err = tx.Exec(`INSERT INTO outbox_events(id,company_id,aggregate_type,aggregate_id,event_type,payload,idempotency_key)
   VALUES(?,?,'marketing_schedule',?,?,?,?)`, eventID, current.CompanyID, current.ID.String(), current.EventType, string(current.Payload), key); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO marketing_job_runs(id,company_id,schedule_id,scheduled_for,outbox_event_id)
   VALUES(?,?,?,?,?)`, uuid.New(), current.CompanyID, current.ID, current.NextRunAt, eventID); err != nil {
			return err
		}
		_, err = tx.Model("marketing_schedules").Where("id", current.ID).Where("company_id", current.CompanyID).
			Data(g.Map{"next_run_at": next, "status": status, "leased_by": nil, "leased_until": nil, "last_error": nil, "updated_at": now}).Update()
		created = err == nil
		return err
	})
	return created, err
}
