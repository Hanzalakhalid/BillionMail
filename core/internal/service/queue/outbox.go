package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const OutboxTaskType = "platform:outbox:deliver"

type Event struct {
	ID             uuid.UUID       `json:"id"`
	CompanyID      uuid.UUID       `json:"company_id"`
	AggregateType  string          `json:"aggregate_type"`
	AggregateID    string          `json:"aggregate_id"`
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	IdempotencyKey string          `json:"idempotency_key"`
	Attempts       int             `json:"attempts"`
}

type Publisher interface {
	Publish(context.Context, Event) error
}

type Dispatcher struct {
	Publisher   Publisher
	WorkerID    string
	BatchSize   int
	Lease       time.Duration
	MaxAttempts int
	Now         func() time.Time
}

func (d Dispatcher) Dispatch(ctx context.Context) (int, error) {
	if d.Publisher == nil {
		return 0, errors.New("outbox publisher is required")
	}
	if strings.TrimSpace(d.WorkerID) == "" {
		return 0, errors.New("outbox worker ID is required")
	}
	if d.BatchSize <= 0 || d.BatchSize > 500 {
		d.BatchSize = 100
	}
	if d.Lease <= 0 {
		d.Lease = 2 * time.Minute
	}
	if d.MaxAttempts <= 0 {
		d.MaxAttempts = 20
	}
	if d.Now == nil {
		d.Now = time.Now
	}

	events, err := d.claim(ctx)
	if err != nil {
		return 0, err
	}
	published := 0
	var dispatchErrors []error
	for _, event := range events {
		if err = d.Publisher.Publish(ctx, event); err != nil {
			dispatchErrors = append(dispatchErrors, d.fail(ctx, event, err))
			continue
		}
		if _, err = g.DB().Model("outbox_events").Ctx(ctx).Data(g.Map{
			"published_at": d.Now().UTC(), "locked_at": nil, "locked_by": nil, "last_error": nil,
		}).Where("id", event.ID).Where("locked_by", d.WorkerID).WhereNull("published_at").Update(); err != nil {
			dispatchErrors = append(dispatchErrors, fmt.Errorf("mark outbox event %s published: %w", event.ID, err))
			continue
		}
		published++
	}
	return published, errors.Join(dispatchErrors...)
}

func (d Dispatcher) claim(ctx context.Context) ([]Event, error) {
	var events []Event
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		rows, queryErr := tx.Query(`
            WITH candidates AS (
                SELECT id FROM outbox_events
                WHERE published_at IS NULL AND dead_lettered_at IS NULL
                  AND available_at <= NOW()
                  AND (locked_at IS NULL OR locked_at < NOW() - (? * INTERVAL '1 second'))
                ORDER BY available_at, created_at
                FOR UPDATE SKIP LOCKED
                LIMIT ?
            )
            UPDATE outbox_events event
            SET locked_at=NOW(), locked_by=?, attempts=event.attempts+1
            FROM candidates
            WHERE event.id=candidates.id
            RETURNING event.id,event.company_id,event.aggregate_type,event.aggregate_id,
                      event.event_type,event.payload,event.idempotency_key,event.attempts`,
			int64(d.Lease.Seconds()), d.BatchSize, d.WorkerID)
		if queryErr != nil {
			return queryErr
		}
		return rows.Structs(&events)
	})
	return events, err
}

func (d Dispatcher) fail(ctx context.Context, event Event, publishErr error) error {
	message := publishErr.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	data := g.Map{"locked_at": nil, "locked_by": nil, "last_error": message}
	if event.Attempts >= d.MaxAttempts {
		data["dead_lettered_at"] = d.Now().UTC()
	} else {
		backoff := time.Minute * time.Duration(1<<min(event.Attempts-1, 8))
		data["available_at"] = d.Now().UTC().Add(backoff)
	}
	_, err := g.DB().Model("outbox_events").Ctx(ctx).Data(data).
		Where("id", event.ID).Where("locked_by", d.WorkerID).WhereNull("published_at").Update()
	if err != nil {
		return fmt.Errorf("publish outbox event %s: %v; release claim: %w", event.ID, publishErr, err)
	}
	return fmt.Errorf("publish outbox event %s: %w", event.ID, publishErr)
}

type AsynqPublisher struct {
	Client *asynq.Client
}

func (p AsynqPublisher) Publish(ctx context.Context, event Event) error {
	if p.Client == nil {
		return errors.New("asynq client is required")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	queueName := workloadQueue(event.EventType)
	_, err = p.Client.EnqueueContext(ctx, asynq.NewTask(OutboxTaskType, payload),
		asynq.TaskID(event.ID.String()), asynq.Queue(queueName), asynq.MaxRetry(12),
		asynq.Timeout(2*time.Minute), asynq.Retention(24*time.Hour))
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

func workloadQueue(eventType string) string {
	switch {
	case strings.HasPrefix(eventType, "identity."), strings.HasPrefix(eventType, "company.invitation."):
		return "notifications"
	case strings.HasPrefix(eventType, "whatsapp.webhook."):
		return "webhooks"
	case strings.HasPrefix(eventType, "whatsapp.message."):
		return "delivery"
	default:
		return "default"
	}
}
