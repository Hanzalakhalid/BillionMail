package workflow

import (
	"context"
	"errors"
	"time"

	"billionmail-core/internal/model/entity"

	"github.com/gogf/gf/v2/frame/g"
)

// Review statuses for manual messages ("Needs review" queue).
const (
	ReviewPending   = "pending"
	ReviewSent      = "sent"
	ReviewFailed    = "failed"
	ReviewDiscarded = "discarded"
)

// Review is a manual message waiting for someone to send or discard it.
type Review struct {
	Id          int64  `json:"id" orm:"id"`
	WorkflowId  int    `json:"workflow_id" orm:"workflow_id"`
	RunId       int64  `json:"run_id" orm:"run_id"`
	ContactId   int    `json:"contact_id" orm:"contact_id"`
	Email       string `json:"email" orm:"email"`
	StepPath    string `json:"step_path" orm:"step_path"`
	StepType    string `json:"step_type" orm:"step_type"`
	Status      string `json:"status" orm:"status"`
	Detail      string `json:"detail" orm:"detail"`
	CreateTime  int    `json:"create_time" orm:"create_time"`
	ResolveTime int    `json:"resolve_time" orm:"resolve_time"`
}

func queueReview(ctx context.Context, workflowId int, run *Run, stepType string) error {
	_, err := g.DB().Model("bm_workflow_reviews").Ctx(ctx).Data(g.Map{
		"workflow_id": workflowId,
		"run_id":      run.Id,
		"contact_id":  run.ContactId,
		"email":       run.Email,
		"step_path":   run.StepPath,
		"step_type":   stepType,
		"status":      ReviewPending,
		"create_time": time.Now().Unix(),
	}).Insert()
	return err
}

// ListReviews pages through a workflow's review queue.
func ListReviews(ctx context.Context, workflowId int, status string, page, pageSize int) (int, []Review, error) {
	m := g.DB().Model("bm_workflow_reviews").Ctx(ctx).Where("workflow_id", workflowId)
	if status != "" {
		m = m.Where("status", status)
	}
	total, err := m.Count()
	if err != nil {
		return 0, nil, err
	}
	var list []Review
	err = m.OrderDesc("id").Page(page, pageSize).Scan(&list)
	if list == nil {
		list = []Review{}
	}
	return total, list, err
}

// ResolveReviews sends or discards pending manual messages. It returns how many were
// sent (or discarded) and how many failed.
func ResolveReviews(ctx context.Context, workflowId int, ids []int64, send bool) (int, int, error) {
	if len(ids) == 0 {
		return 0, 0, errors.New("choose at least one message")
	}
	var items []Review
	err := g.DB().Model("bm_workflow_reviews").Ctx(ctx).
		Where("workflow_id", workflowId).WhereIn("id", ids).Where("status", ReviewPending).Scan(&items)
	if err != nil {
		return 0, 0, err
	}

	if !send {
		_, err = g.DB().Model("bm_workflow_reviews").Ctx(ctx).
			Where("workflow_id", workflowId).WhereIn("id", ids).Where("status", ReviewPending).
			Data(g.Map{"status": ReviewDiscarded, "detail": "discarded", "resolve_time": time.Now().Unix()}).Update()
		return len(items), 0, err
	}

	w, err := Get(ctx, workflowId)
	if err != nil {
		return 0, 0, err
	}
	ok, failed := 0, 0
	for _, it := range items {
		status, detail := sendReviewed(ctx, w, it)
		if status == ReviewSent {
			ok++
		} else {
			failed++
		}
		_, _ = g.DB().Model("bm_workflow_reviews").Ctx(ctx).Where("id", it.Id).
			Data(g.Map{"status": status, "detail": detail, "resolve_time": time.Now().Unix()}).Update()
	}
	return ok, failed, nil
}

// sendReviewed sends one reviewed message with the step as it is now in the workflow.
func sendReviewed(ctx context.Context, w *Workflow, it Review) (string, string) {
	step, found := StepAt(w.Steps, it.StepPath)
	if !found || step.Type != it.StepType {
		return ReviewFailed, "this step was changed or removed"
	}
	var contact entity.Contact
	if err := g.DB().Model("bm_contacts").Ctx(ctx).Where("id", it.ContactId).Scan(&contact); err != nil || contact.Id == 0 {
		return ReviewFailed, "contact was deleted"
	}
	run := &Run{Id: it.RunId, WorkflowId: w.Id, ContactId: it.ContactId, Email: it.Email, StepPath: it.StepPath}
	status, detail, messageId := runAction(ctx, w, step, &contact)
	writeLog(ctx, w.Id, run, step.Type, status, "reviewed: "+detail, messageId)
	if status == LogDone {
		return ReviewSent, detail
	}
	return ReviewFailed, detail
}
