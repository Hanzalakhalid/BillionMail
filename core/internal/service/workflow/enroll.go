package workflow

import (
	"context"
	"fmt"
	"time"

	"billionmail-core/internal/model/entity"

	"github.com/gogf/gf/v2/frame/g"
)

// enrollBatch caps how many contacts one workflow enrolls per tick, so a big group
// is spread over several ticks instead of blocking the timer.
const enrollBatch = 500

// dateScanEvery limits how often date triggers scan contact attributes.
const dateScanEvery = 10 * time.Minute

var lastDateScan time.Time

type candidate struct {
	ContactId int    `orm:"contact_id"`
	Email     string `orm:"email"`
	Key       string `orm:"trigger_key"`
}

// Enroll adds newly qualifying contacts to every active workflow.
func Enroll(ctx context.Context) {
	var rows []row
	if err := g.DB().Model("bm_workflows").Ctx(ctx).Where("status", StatusActive).Scan(&rows); err != nil {
		g.Log().Warning(ctx, "[workflow] load active workflows:", err)
		return
	}

	scanDates := time.Since(lastDateScan) >= dateScanEvery
	for _, r := range rows {
		w := r.toWorkflow()
		var (
			cands []candidate
			err   error
		)
		switch w.Trigger.Type {
		case TriggerGroupJoin:
			cands, err = groupJoinCandidates(ctx, w)
		case TriggerTagAdded:
			cands, err = tagAddedCandidates(ctx, w)
		case TriggerEmailOpened:
			cands, err = emailEventCandidates(ctx, w, "mailstat_opened", "open")
		case TriggerEmailClicked:
			cands, err = emailEventCandidates(ctx, w, "mailstat_clicked", "click")
		case TriggerDate:
			if scanDates {
				cands, err = dateCandidates(ctx, w, time.Now())
			}
		}
		if err != nil {
			g.Log().Warningf(ctx, "[workflow %d] find contacts: %v", w.Id, err)
			continue
		}
		if n := insertRuns(ctx, w, cands); n > 0 {
			g.Log().Infof(ctx, "[workflow %d] %d contacts entered", w.Id, n)
		}
	}
	if scanDates {
		lastDateScan = time.Now()
	}
}

// groupJoinCandidates: subscribed contacts added to the group after enroll_since.
func groupJoinCandidates(ctx context.Context, w Workflow) ([]candidate, error) {
	var out []candidate
	err := g.DB().GetScan(ctx, &out, `
		SELECT c.id AS contact_id, c.email, 'join' AS trigger_key
		FROM bm_contacts c
		WHERE c.group_id = ? AND c.active = 1 AND c.create_time >= ?
		  AND NOT EXISTS (SELECT 1 FROM bm_workflow_runs r
		                  WHERE r.workflow_id = ? AND r.contact_id = c.id AND r.trigger_key = 'join')
		ORDER BY c.id LIMIT ?`,
		w.Trigger.GroupId, w.EnrollSince, w.Id, enrollBatch)
	return out, err
}

// tagAddedCandidates: contacts that got the tag after enroll_since. Removing and
// adding the tag again starts the workflow again (new tag row = new key).
func tagAddedCandidates(ctx context.Context, w Workflow) ([]candidate, error) {
	var out []candidate
	err := g.DB().GetScan(ctx, &out, `
		SELECT c.id AS contact_id, c.email, 'tag:' || ct.create_time AS trigger_key
		FROM bm_contact_tags ct
		JOIN bm_contacts c ON c.id = ct.contact_id
		WHERE ct.tag_id = ? AND ct.create_time >= ?
		  AND NOT EXISTS (SELECT 1 FROM bm_workflow_runs r
		                  WHERE r.workflow_id = ? AND r.contact_id = c.id
		                    AND r.trigger_key = 'tag:' || ct.create_time)
		ORDER BY ct.id LIMIT ?`,
		w.Trigger.TagId, w.EnrollSince, w.Id, enrollBatch)
	return out, err
}

// emailEventCandidates: contacts who opened/clicked the campaign after enroll_since.
// The subscribed contact row with that email is used (lowest id when it is in several groups).
func emailEventCandidates(ctx context.Context, w Workflow, table, prefix string) ([]candidate, error) {
	key := fmt.Sprintf("%s:%d", prefix, w.Trigger.TaskId)
	var out []candidate
	err := g.DB().GetScan(ctx, &out, `
		SELECT DISTINCT ON (c.email) c.id AS contact_id, c.email, ? AS trigger_key
		FROM (SELECT DISTINCT recipient FROM `+table+` WHERE campaign_id = ? AND log_time >= ?) e
		JOIN bm_contacts c ON c.email = e.recipient AND c.active = 1
		WHERE NOT EXISTS (SELECT 1 FROM bm_workflow_runs r
		                  WHERE r.workflow_id = ? AND r.email = c.email AND r.trigger_key = ?)
		ORDER BY c.email, c.id LIMIT ?`,
		key, w.Trigger.TaskId, w.EnrollSince, w.Id, key, enrollBatch)
	return out, err
}

// dateCandidates: contacts whose date attribute (+ offset) falls on today.
func dateCandidates(ctx context.Context, w Workflow, now time.Time) ([]candidate, error) {
	type dateRow struct {
		Id    int    `orm:"id"`
		Email string `orm:"email"`
		Value string `orm:"value"`
	}
	sql := `SELECT id, email, attribs->>? AS value FROM bm_contacts
		WHERE active = 1 AND COALESCE(attribs->>?, '') <> ''`
	args := []interface{}{w.Trigger.Attribute, w.Trigger.Attribute}
	if w.Trigger.GroupId > 0 {
		sql += " AND group_id = ?"
		args = append(args, w.Trigger.GroupId)
	}
	var rows []dateRow
	if err := g.DB().GetScan(ctx, &rows, sql, args...); err != nil {
		return nil, err
	}

	var out []candidate
	for _, r := range rows {
		key, ok := DateTriggerKey(r.Value, w.Trigger.OffsetDays, w.Trigger.Yearly, w.Trigger.Monthly, now)
		if ok {
			out = append(out, candidate{ContactId: r.Id, Email: r.Email, Key: key})
		}
	}
	return out, nil
}

// insertRuns starts each candidate at step 0. Candidates that fail the flow filters or the
// re-entry rule are recorded as skipped, so they are not picked up again every tick.
// Duplicates are ignored by the unique key.
func insertRuns(ctx context.Context, w Workflow, cands []candidate) int {
	now := time.Now().Unix()
	n := 0
	for _, c := range cands {
		status, reason := RunActive, ""
		if why, ok := blockedByReentry(ctx, w, c.Email, now); !ok {
			status, reason = RunSkipped, why
		} else if why, ok := passesFlowFilters(ctx, w, c.ContactId); !ok {
			status, reason = RunSkipped, why
		}
		res, err := g.DB().Ctx(ctx).Exec(ctx, `
			INSERT INTO bm_workflow_runs (workflow_id, contact_id, email, trigger_key, step_index, step_path, status, next_run_at, last_error, create_time, update_time)
			VALUES (?, ?, ?, ?, 0, '0', ?, ?, ?, ?, ?)
			ON CONFLICT (workflow_id, contact_id, trigger_key) DO NOTHING`,
			w.Id, c.ContactId, c.Email, c.Key, status, now, reason, now, now)
		if err != nil {
			g.Log().Warning(ctx, fmt.Sprintf("[workflow %d] enroll contact %d:", w.Id, c.ContactId), err)
			continue
		}
		if affected, _ := res.RowsAffected(); affected > 0 && status == RunActive {
			n++
		}
	}
	return n
}

// blockedByReentry applies the trigger's re-entry rule. Returns ok=false with a reason when
// the contact may not start again.
func blockedByReentry(ctx context.Context, w Workflow, email string, now int64) (string, bool) {
	m := g.DB().Model("bm_workflow_runs").Ctx(ctx).
		Where("workflow_id", w.Id).Where("email", email).WhereNot("status", RunSkipped)
	switch w.Trigger.Reentry {
	case ReentryOnce:
		if n, err := m.Count(); err == nil && n > 0 {
			return "already went through this workflow (enter only once)", false
		}
	case ReentryAfter:
		since := now - int64(w.Trigger.ReentryDays)*86400
		if n, err := m.WhereGTE("create_time", since).Count(); err == nil && n > 0 {
			return fmt.Sprintf("entered less than %d days ago", w.Trigger.ReentryDays), false
		}
	}
	return "", true
}

// passesFlowFilters checks the workflow's flow filters when someone enters.
func passesFlowFilters(ctx context.Context, w Workflow, contactId int) (string, bool) {
	if len(w.Settings.FlowFilters) == 0 {
		return "", true
	}
	var contact entity.Contact
	if err := g.DB().Model("bm_contacts").Ctx(ctx).Where("id", contactId).Scan(&contact); err != nil || contact.Id == 0 {
		return "contact not found", false
	}
	for _, f := range w.Settings.FlowFilters {
		if ok, why := checkCondition(ctx, f, contact, nil); !ok {
			return "did not match flow filters: " + why, false
		}
	}
	return "", true
}
