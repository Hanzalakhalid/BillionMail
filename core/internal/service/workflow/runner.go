package workflow

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"billionmail-core/internal/model/entity"

	"github.com/gogf/gf/v2/frame/g"
)

// runBatch caps how many due contacts are processed per tick.
const runBatch = 200

var processMu sync.Mutex

// Tick enrolls new contacts and moves due contacts forward. Called by a timer;
// overlapping calls are skipped.
func Tick(ctx context.Context) {
	if !processMu.TryLock() {
		return
	}
	defer processMu.Unlock()

	Enroll(ctx)
	ProcessDue(ctx)
}

// ProcessDue runs the next steps for every contact whose wait is over.
func ProcessDue(ctx context.Context) {
	var runs []Run
	err := g.DB().GetScan(ctx, &runs, `
		SELECT r.* FROM bm_workflow_runs r
		JOIN bm_workflows w ON w.id = r.workflow_id
		WHERE r.status = ? AND r.next_run_at <= ? AND w.status = ?
		ORDER BY r.next_run_at LIMIT ?`,
		RunActive, time.Now().Unix(), StatusActive, runBatch)
	if err != nil {
		g.Log().Warning(ctx, "[workflow] load due runs:", err)
		return
	}
	if len(runs) == 0 {
		return
	}

	workflows := map[int]*Workflow{}
	for i := range runs {
		w, ok := workflows[runs[i].WorkflowId]
		if !ok {
			w, err = Get(ctx, runs[i].WorkflowId)
			if err != nil {
				w = nil
			}
			workflows[runs[i].WorkflowId] = w
		}
		if w == nil {
			continue
		}
		advance(ctx, w, &runs[i])
	}
}

// startPath is where a run is now; runs created before branches existed only have step_index.
func startPath(run *Run) string {
	if run.StepPath != "" {
		return run.StepPath
	}
	return RootPath(run.StepIndex)
}

// advance executes steps for one contact until it hits a wait, exits or finishes.
func advance(ctx context.Context, w *Workflow, run *Run) {
	var contact entity.Contact
	err := g.DB().Model("bm_contacts").Ctx(ctx).Where("id", run.ContactId).Scan(&contact)
	if err != nil || contact.Id == 0 {
		run.StepPath = startPath(run)
		writeLog(ctx, w.Id, run, StepCondition, LogExited, "contact was deleted", "")
		saveRun(ctx, run, RunExited, run.StepPath, 0, "contact was deleted")
		return
	}

	path := startPath(run)
	for guard := 0; guard <= MaxSteps; guard++ {
		step, ok := StepAt(w.Steps, path)
		if !ok {
			saveRun(ctx, run, RunCompleted, path, 0, run.LastError)
			return
		}
		run.StepPath = path

		// Goal reached? (e.g. booked an appointment) -> leave before doing anything else.
		for _, c := range w.Settings.ExitConditions {
			if met, why := checkCondition(ctx, c, contact, run); met {
				reason := "goal reached: " + why
				writeLog(ctx, w.Id, run, step.Type, LogExited, reason, "")
				saveRun(ctx, run, RunExited, path, 0, "")
				return
			}
		}

		switch step.Type {
		case StepWait:
			until := WaitUntil(step, time.Now())
			detail := "waiting " + waitText(step.WaitValue, step.WaitUnit)
			if step.AtTime != "" || len(step.Weekdays) > 0 {
				detail += ", until " + until.Format("Mon 2 Jan 15:04 MST")
			}
			writeLog(ctx, w.Id, run, step.Type, LogDone, detail, "")
			saveRun(ctx, run, RunActive, NextPath(w.Steps, path), until.Unix(), "")
			return

		case StepCondition:
			met, why := checkCondition(ctx, step.Cond, contact, run)
			if !met {
				writeLog(ctx, w.Id, run, step.Type, LogExited, "stopped: "+why, "")
				saveRun(ctx, run, RunExited, path, 0, "")
				return
			}
			writeLog(ctx, w.Id, run, step.Type, LogDone, why, "")
			path = NextPath(w.Steps, path)
			continue

		case StepSplit:
			branch, why := chooseBranch(ctx, step, contact, run)
			writeLog(ctx, w.Id, run, step.Type, LogDone, why, "")
			path = EnterBranch(w.Steps, path, branch)
			continue
		}

		if step.IsMessage() {
			// Klaviyo order: draft -> filters -> quiet hours -> smart sending -> manual/live.
			if step.Status == MessageDraft {
				writeLog(ctx, w.Id, run, step.Type, LogSkipped, "message is a draft", "")
				path = NextPath(w.Steps, path)
				continue
			}
			if ok, why := passesFilters(ctx, w.Settings.FlowFilters, step.Filters, contact, run); !ok {
				writeLog(ctx, w.Id, run, step.Type, LogSkipped, why, "")
				path = NextPath(w.Steps, path)
				continue
			}
			// Quiet hours: hold the message until the sending window opens.
			if at := NextSendTime(w.Settings.QuietHours, time.Now()); at.After(time.Now()) {
				writeLog(ctx, w.Id, run, step.Type, LogWaiting, "quiet hours: sends at "+at.Format("Mon 15:04 MST"), "")
				saveRun(ctx, run, RunActive, path, at.Unix(), "")
				return
			}
			// Smart sending: don't message the same person again too soon.
			if h := w.Settings.SmartSendingHours; h > 0 && !step.SkipSmartSending && recentlyMessaged(ctx, contact.Email, h) {
				writeLog(ctx, w.Id, run, step.Type, LogSkipped, fmt.Sprintf("smart sending: already got a message in the last %d hours", h), "")
				path = NextPath(w.Steps, path)
				continue
			}
			if step.Status == MessageManual {
				if err := queueReview(ctx, w.Id, run, step.Type); err != nil {
					writeLog(ctx, w.Id, run, step.Type, LogFailed, "could not queue for review: "+err.Error(), "")
				} else {
					writeLog(ctx, w.Id, run, step.Type, LogReview, "waiting for review (manual message)", "")
				}
				path = NextPath(w.Steps, path)
				continue
			}
		}

		status, detail, messageId := runAction(ctx, w, step, &contact)
		writeLog(ctx, w.Id, run, step.Type, status, detail, messageId)
		if status == LogFailed {
			run.LastError = detail
		}
		path = NextPath(w.Steps, path)
	}
	saveRun(ctx, run, RunCompleted, path, 0, run.LastError)
}

// chooseBranch picks Yes ("y") or No ("n") for a split; for A/B, A is Yes.
func chooseBranch(ctx context.Context, step Step, contact entity.Contact, run *Run) (string, string) {
	if step.SplitMode == SplitRandom {
		if rand.Intn(100) < step.Percent {
			return BranchYes, fmt.Sprintf("A/B test: went to A (%d%%)", step.Percent)
		}
		return BranchNo, fmt.Sprintf("A/B test: went to B (%d%%)", 100-step.Percent)
	}
	met, why := checkCondition(ctx, step.Cond, contact, run)
	if met {
		return BranchYes, "Yes: " + why
	}
	return BranchNo, "No: " + why
}

// recentlyMessaged reports whether any workflow sent this email address a message in the last hours.
func recentlyMessaged(ctx context.Context, email string, hours int) bool {
	n, err := g.DB().Model("bm_workflow_logs").Ctx(ctx).
		Where("email", email).
		Where("status", LogDone).
		WhereIn("step_type", []string{StepEmail, StepSMS, StepWhatsApp}).
		WhereGTE("create_time", time.Now().Add(-time.Duration(hours)*time.Hour).Unix()).
		Count()
	return err == nil && n > 0
}

// waitText formats a wait like "1 day" or "3 hours".
func waitText(value int, unit string) string {
	if unit == "" {
		unit = "days"
	}
	if value == 1 {
		unit = strings.TrimSuffix(unit, "s")
	}
	return fmt.Sprintf("%d %s", value, unit)
}

// saveRun stores where the contact is now. An empty path with an active status means
// the workflow has no more steps, so the run is completed instead.
func saveRun(ctx context.Context, run *Run, status, path string, nextRunAt int64, lastError string) {
	if status == RunActive && path == "" {
		status = RunCompleted
	}
	data := g.Map{
		"status":      status,
		"step_path":   path,
		"step_index":  TopIndex(path),
		"last_error":  lastError,
		"update_time": time.Now().Unix(),
	}
	if nextRunAt > 0 {
		data["next_run_at"] = nextRunAt
	}
	if _, err := g.DB().Model("bm_workflow_runs").Ctx(ctx).Where("id", run.Id).Data(data).Update(); err != nil {
		g.Log().Warningf(ctx, "[workflow %d] save run %d: %v", run.WorkflowId, run.Id, err)
	}
}

func writeLog(ctx context.Context, workflowId int, run *Run, stepType, status, detail, messageId string) {
	_, err := g.DB().Model("bm_workflow_logs").Ctx(ctx).Data(g.Map{
		"workflow_id": workflowId,
		"run_id":      run.Id,
		"contact_id":  run.ContactId,
		"email":       run.Email,
		"step_index":  TopIndex(run.StepPath),
		"step_path":   run.StepPath,
		"step_type":   stepType,
		"status":      status,
		"detail":      detail,
		"message_id":  messageId,
		"create_time": time.Now().Unix(),
	}).Insert()
	if err != nil {
		g.Log().Warningf(ctx, "[workflow %d] write log: %v", workflowId, err)
	}
}

// passesFilters checks the flow filters and the message's own filters; all must be true.
func passesFilters(ctx context.Context, flow, message []Cond, contact entity.Contact, run *Run) (bool, string) {
	for _, f := range flow {
		if ok, why := checkCondition(ctx, f, contact, run); !ok {
			return false, "skipped: does not match flow filters (" + why + ")"
		}
	}
	for _, f := range message {
		if ok, why := checkCondition(ctx, f, contact, run); !ok {
			return false, "skipped: does not match message filters (" + why + ")"
		}
	}
	return true, ""
}
