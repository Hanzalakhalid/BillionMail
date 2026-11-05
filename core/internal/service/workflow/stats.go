package workflow

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

// StepStats is what happened at one step, for the per-message report.
type StepStats struct {
	Done    int `json:"done"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	Exited  int `json:"exited"`
	Review  int `json:"review"`  // manual messages queued for review
	Waiting int `json:"waiting"` // currently waiting at this step
	Opened  int `json:"opened"`  // email steps: unique opens
	Clicked int `json:"clicked"` // email steps: unique clicks
}

// GetStepStats returns stats per step path for a workflow.
func GetStepStats(ctx context.Context, workflowId int) (map[string]*StepStats, error) {
	out := map[string]*StepStats{}
	get := func(path string) *StepStats {
		s := out[path]
		if s == nil {
			s = &StepStats{}
			out[path] = s
		}
		return s
	}

	type statusRow struct {
		StepPath string `orm:"step_path"`
		Status   string `orm:"status"`
		Cnt      int    `orm:"cnt"`
	}
	var rows []statusRow
	err := g.DB().GetScan(ctx, &rows, `
		SELECT step_path, status, COUNT(*) AS cnt FROM bm_workflow_logs
		WHERE workflow_id = ? AND step_path <> '' AND status <> ?
		GROUP BY step_path, status`, workflowId, LogWaiting)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		s := get(r.StepPath)
		switch r.Status {
		case LogDone:
			s.Done = r.Cnt
		case LogSkipped:
			s.Skipped = r.Cnt
		case LogFailed:
			s.Failed = r.Cnt
		case LogExited:
			s.Exited = r.Cnt
		case LogReview:
			s.Review = r.Cnt
		}
	}

	type countRow struct {
		StepPath string `orm:"step_path"`
		Cnt      int    `orm:"cnt"`
	}
	var waiting []countRow
	err = g.DB().GetScan(ctx, &waiting, `
		SELECT step_path, COUNT(*) AS cnt FROM bm_workflow_runs
		WHERE workflow_id = ? AND status = ? AND step_path <> ''
		GROUP BY step_path`, workflowId, RunActive)
	if err != nil {
		return nil, err
	}
	for _, r := range waiting {
		get(r.StepPath).Waiting = r.Cnt
	}

	for _, t := range []struct {
		table string
		set   func(*StepStats, int)
	}{
		{"mailstat_opened", func(s *StepStats, n int) { s.Opened = n }},
		{"mailstat_clicked", func(s *StepStats, n int) { s.Clicked = n }},
	} {
		var rows []countRow
		err = g.DB().GetScan(ctx, &rows, `
			SELECT l.step_path, COUNT(DISTINCT l.id) AS cnt FROM bm_workflow_logs l
			WHERE l.workflow_id = ? AND l.message_id <> ''
			  AND EXISTS (SELECT 1 FROM `+t.table+` m WHERE m.message_id = l.message_id)
			GROUP BY l.step_path`, workflowId)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			t.set(get(r.StepPath), r.Cnt)
		}
	}
	return out, nil
}
