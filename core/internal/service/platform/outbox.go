package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

type OutboxEvent struct {
	AggregateType  string
	AggregateID    string
	EventType      string
	Payload        any
	IdempotencyKey string
	AvailableAt    time.Time
}

type QueuePublisher interface {
	PublishOutbox(context.Context, tenancy.TenantContext, OutboxEvent) error
}

type DatabaseOutbox struct{}

func (DatabaseOutbox) PublishOutbox(ctx context.Context, tenant tenancy.TenantContext, event OutboxEvent) error {
	if tenant.CompanyID == uuid.Nil || event.AggregateType == "" || event.AggregateID == "" || event.EventType == "" || event.IdempotencyKey == "" {
		return fmt.Errorf("outbox aggregate, event type, and idempotency key are required")
	}
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("encode outbox payload: %w", err)
	}
	if event.AvailableAt.IsZero() {
		event.AvailableAt = time.Now().UTC()
	}
	_, err = g.DB().Model("outbox_events").Data(g.Map{
		"id": uuid.New(), "company_id": tenant.CompanyID, "aggregate_type": event.AggregateType,
		"aggregate_id": event.AggregateID, "event_type": event.EventType, "payload": payload,
		"idempotency_key": event.IdempotencyKey, "available_at": event.AvailableAt,
	}).Insert()
	return err
}
