package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// RecordTaskFailure mirrors terminal Asynq failures into PostgreSQL so they
// survive Redis inspection/retention and can be replayed with an audit trail.
func RecordTaskFailure(ctx context.Context, task *asynq.Task, failure error) error {
	var event Event
	if task == nil || task.Type() != OutboxTaskType || json.Unmarshal(task.Payload(), &event) != nil || event.ID == uuid.Nil || event.CompanyID == uuid.Nil {
		return nil
	}
	retried, hasRetries := asynq.GetRetryCount(ctx)
	maxRetry, hasMax := asynq.GetMaxRetry(ctx)
	terminal := errors.Is(failure, asynq.SkipRetry) || (hasRetries && hasMax && retried >= maxRetry)
	return recordFailure(ctx, event, terminal)
}

func recordFailure(ctx context.Context, event Event, terminal bool) error {
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
			return err
		}
		now := time.Now().UTC()
		// Provider errors may embed credentials or personal data. Store a stable
		// classification here rather than copying arbitrary provider response text.
		data := g.Map{"last_error": "worker execution failed"}
		status := "retrying"
		if terminal {
			data["dead_lettered_at"] = now
			status = "dead-lettered"
		}
		result, err := tx.Model("outbox_events").Where("id", event.ID).Where("company_id", event.CompanyID).Where("dispatch_generation", event.DispatchGeneration).Data(data).Update()
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed == 0 {
			return err
		}
		_, err = tx.Model("marketing_job_runs").Where("company_id", event.CompanyID).Where("outbox_event_id", event.ID).Data(g.Map{"status": status, "updated_at": now}).Update()
		return err
	})
}

// Replay retains the event identity, so idempotent handlers and delivery
// mappings continue to recognize the original operation. A new dispatch
// generation avoids collision with the archived Asynq task's ID.
func Replay(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID) error {
	if err := tenancy.Authorize(tenant, "job.manage"); err != nil {
		return err
	}
	if tenant.CompanyID == uuid.Nil || tenant.UserID == uuid.Nil || id == uuid.Nil {
		return errors.New("company, actor and event are required")
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		result, err := tx.Model("outbox_events").Where("id", id).Where("company_id", tenant.CompanyID).WhereNotNull("dead_lettered_at").
			Where("NOT EXISTS (SELECT 1 FROM outbox_deliveries d WHERE d.event_id=outbox_events.id)").
			Data(g.Map{"dispatch_generation": gdb.Raw("dispatch_generation+1"), "published_at": nil, "dead_lettered_at": nil, "locked_at": nil, "locked_by": nil, "attempts": 0, "last_error": nil, "available_at": time.Now().UTC()}).Update()
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return errors.New("dead letter is unavailable or already replayed")
		}
		if _, err = tx.Model("marketing_job_runs").Where("company_id", tenant.CompanyID).Where("outbox_event_id", id).Data(g.Map{"status": "queued", "updated_at": time.Now().UTC()}).Update(); err != nil {
			return err
		}
		_, err = tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "job.dead_letter.replayed", "resource_type": "outbox_event", "resource_id": id.String()}).Insert()
		return err
	})
}
