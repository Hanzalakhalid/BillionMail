package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type Handler func(context.Context, Event) error

type Processor struct {
	Consumer string
	Handlers map[string]Handler
}

func (p Processor) HandleTask(ctx context.Context, task *asynq.Task) error {
	if p.Consumer == "" {
		return errors.New("outbox consumer name is required")
	}
	var event Event
	if err := json.Unmarshal(task.Payload(), &event); err != nil {
		return fmt.Errorf("decode outbox event: %w", asynq.SkipRetry)
	}
	if event.ID == uuid.Nil || event.CompanyID == uuid.Nil || event.EventType == "" {
		return fmt.Errorf("invalid outbox event: %w", asynq.SkipRetry)
	}
	var stored struct {
		Event
		DeadLetteredAt *time.Time `json:"dead_lettered_at"`
	}
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
			return err
		}
		return tx.Model("outbox_events").Where("id", event.ID).Where("company_id", event.CompanyID).Scan(&stored)
	})
	if err != nil {
		return err
	}
	if stored.ID == uuid.Nil {
		return fmt.Errorf("outbox event missing: %w", asynq.SkipRetry)
	}
	if stored.DispatchGeneration != event.DispatchGeneration || stored.DeadLetteredAt != nil {
		return nil
	}
	// Redis carries a reference envelope; PostgreSQL owns the command content.
	event = stored.Event
	completed, err := g.DB().Model("outbox_deliveries").Ctx(ctx).
		Where("event_id", event.ID).Where("company_id", event.CompanyID).Where("consumer", p.Consumer).Count()
	if err != nil {
		return err
	}
	if completed > 0 {
		return nil
	}
	handler, exists := p.Handlers[event.EventType]
	if !exists {
		return fmt.Errorf("no handler registered for %s: %w", event.EventType, asynq.SkipRetry)
	}
	if err = handler(ctx, event); err != nil {
		return err
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); txErr != nil {
			return txErr
		}
		_, txErr := tx.Exec(`INSERT INTO outbox_deliveries
            (id,company_id,event_id,consumer,completed_at)
            VALUES(?,?,?,?,?) ON CONFLICT(event_id,consumer) DO NOTHING`,
			uuid.New(), event.CompanyID, event.ID, p.Consumer, time.Now().UTC())
		if txErr != nil {
			return txErr
		}
		_, txErr = tx.Model("marketing_job_runs").Where("company_id", event.CompanyID).Where("outbox_event_id", event.ID).
			Data(g.Map{"status": "succeeded", "updated_at": time.Now().UTC()}).Update()
		return txErr
	})
}
