package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Workflow is a saved automation flow.
type Workflow struct {
	Id          int      `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Trigger     Trigger  `json:"trigger"`
	Steps       []Step   `json:"steps"`
	Settings    Settings `json:"settings"`
	EnrollSince int      `json:"enroll_since"`
	CreateTime  int      `json:"create_time"`
	UpdateTime  int      `json:"update_time"`
}

// Stats counts contacts per run status.
type Stats struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Completed int `json:"completed"`
	Exited    int `json:"exited"`
	Skipped   int `json:"skipped"` // didn't start: flow filters or re-entry rule
}

// ListItem is a workflow with its stats.
type ListItem struct {
	Workflow
	Stats Stats `json:"stats"`
}

type row struct {
	Id            int    `orm:"id"`
	Name          string `orm:"name"`
	Status        string `orm:"status"`
	TriggerType   string `orm:"trigger_type"`
	TriggerConfig string `orm:"trigger_config"`
	Steps         string `orm:"steps"`
	Settings      string `orm:"settings"`
	EnrollSince   int    `orm:"enroll_since"`
	CreateTime    int    `orm:"create_time"`
	UpdateTime    int    `orm:"update_time"`
}

func (r row) toWorkflow() Workflow {
	w := Workflow{
		Id:          r.Id,
		Name:        r.Name,
		Status:      r.Status,
		EnrollSince: r.EnrollSince,
		CreateTime:  r.CreateTime,
		UpdateTime:  r.UpdateTime,
		Steps:       []Step{},
	}
	_ = json.Unmarshal([]byte(r.TriggerConfig), &w.Trigger)
	w.Trigger.Type = r.TriggerType
	_ = json.Unmarshal([]byte(r.Steps), &w.Steps)
	if r.Settings != "" {
		_ = json.Unmarshal([]byte(r.Settings), &w.Settings)
	}
	if w.Settings.ExitConditions == nil {
		w.Settings.ExitConditions = []Cond{}
	}
	return w
}

func validate(name string, trigger Trigger, steps []Step, settings Settings) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("enter a workflow name")
	}
	if err := trigger.Validate(); err != nil {
		return err
	}
	if err := settings.Validate(); err != nil {
		return err
	}
	return ValidateSteps(steps)
}

func encode(trigger Trigger, steps []Step, settings Settings) (string, string, string, error) {
	if settings.ExitConditions == nil {
		settings.ExitConditions = []Cond{}
	}
	t, err := json.Marshal(trigger)
	if err != nil {
		return "", "", "", err
	}
	s, err := json.Marshal(steps)
	if err != nil {
		return "", "", "", err
	}
	st, err := json.Marshal(settings)
	if err != nil {
		return "", "", "", err
	}
	return string(t), string(s), string(st), nil
}

// List returns all workflows, newest first, with how many contacts are in each.
func List(ctx context.Context) ([]ListItem, error) {
	var rows []row
	if err := g.DB().Model("bm_workflows").Ctx(ctx).OrderDesc("id").Scan(&rows); err != nil {
		return nil, err
	}

	type countRow struct {
		WorkflowId int    `orm:"workflow_id"`
		Status     string `orm:"status"`
		Cnt        int    `orm:"cnt"`
	}
	var counts []countRow
	err := g.DB().Model("bm_workflow_runs").Ctx(ctx).
		Fields("workflow_id, status, COUNT(*) AS cnt").
		Group("workflow_id, status").
		Scan(&counts)
	if err != nil {
		return nil, err
	}
	stats := map[int]*Stats{}
	for _, c := range counts {
		s := stats[c.WorkflowId]
		if s == nil {
			s = &Stats{}
			stats[c.WorkflowId] = s
		}
		if c.Status != RunSkipped {
			s.Total += c.Cnt
		}
		switch c.Status {
		case RunSkipped:
			s.Skipped += c.Cnt
		case RunActive:
			s.Active += c.Cnt
		case RunCompleted:
			s.Completed += c.Cnt
		case RunExited:
			s.Exited += c.Cnt
		}
	}

	list := make([]ListItem, 0, len(rows))
	for _, r := range rows {
		item := ListItem{Workflow: r.toWorkflow()}
		if s := stats[r.Id]; s != nil {
			item.Stats = *s
		}
		list = append(list, item)
	}
	return list, nil
}

// Get loads one workflow.
func Get(ctx context.Context, id int) (*Workflow, error) {
	var r row
	err := g.DB().Model("bm_workflows").Ctx(ctx).Where("id", id).Scan(&r)
	if err != nil {
		return nil, err
	}
	if r.Id == 0 {
		return nil, errors.New("workflow not found")
	}
	w := r.toWorkflow()
	return &w, nil
}

// Create saves a new workflow as a draft.
func Create(ctx context.Context, name string, trigger Trigger, steps []Step, settings Settings) (int, error) {
	if err := validate(name, trigger, steps, settings); err != nil {
		return 0, err
	}
	t, s, st, err := encode(trigger, steps, settings)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	id, err := g.DB().Model("bm_workflows").Ctx(ctx).Data(g.Map{
		"name":           strings.TrimSpace(name),
		"status":         StatusDraft,
		"trigger_type":   trigger.Type,
		"trigger_config": t,
		"steps":          s,
		"settings":       st,
		"create_time":    now,
		"update_time":    now,
	}).InsertAndGetId()
	return int(id), err
}

// Update changes a workflow. Contacts already inside keep their position (step number).
func Update(ctx context.Context, id int, name string, trigger Trigger, steps []Step, settings Settings) error {
	if err := validate(name, trigger, steps, settings); err != nil {
		return err
	}
	if _, err := Get(ctx, id); err != nil {
		return err
	}
	t, s, st, err := encode(trigger, steps, settings)
	if err != nil {
		return err
	}
	_, err = g.DB().Model("bm_workflows").Ctx(ctx).Where("id", id).Data(g.Map{
		"name":           strings.TrimSpace(name),
		"trigger_type":   trigger.Type,
		"trigger_config": t,
		"steps":          s,
		"settings":       st,
		"update_time":    time.Now().Unix(),
	}).Update()
	return err
}

// Delete removes a workflow with its runs and logs.
func Delete(ctx context.Context, id int) error {
	_, err := g.DB().Model("bm_workflows").Ctx(ctx).Where("id", id).Delete()
	return err
}

// SetStatus turns a workflow on (active) or off (paused).
// The first activation decides who can enter: everyone already in the group/tag when
// "include existing" is on, otherwise only contacts that enter from now on.
func SetStatus(ctx context.Context, id int, status string) error {
	if status != StatusActive && status != StatusPaused {
		return errors.New("status must be active or paused")
	}
	w, err := Get(ctx, id)
	if err != nil {
		return err
	}
	data := g.Map{"status": status, "update_time": time.Now().Unix()}
	if status == StatusActive && w.Status == StatusDraft {
		since := time.Now().Unix()
		if w.Trigger.IncludeExisting {
			since = 0
		}
		data["enroll_since"] = since
	}
	_, err = g.DB().Model("bm_workflows").Ctx(ctx).Where("id", id).Data(data).Update()
	return err
}

// Run is one contact's progress through a workflow.
type Run struct {
	Id         int64  `json:"id" orm:"id"`
	WorkflowId int    `json:"workflow_id" orm:"workflow_id"`
	ContactId  int    `json:"contact_id" orm:"contact_id"`
	Email      string `json:"email" orm:"email"`
	TriggerKey string `json:"trigger_key" orm:"trigger_key"`
	StepIndex  int    `json:"step_index" orm:"step_index"`
	StepPath   string `json:"step_path" orm:"step_path"`
	Status     string `json:"status" orm:"status"`
	NextRunAt  int    `json:"next_run_at" orm:"next_run_at"`
	LastError  string `json:"last_error" orm:"last_error"`
	CreateTime int    `json:"create_time" orm:"create_time"`
	UpdateTime int    `json:"update_time" orm:"update_time"`
}

// ListRuns pages through the contacts in a workflow.
func ListRuns(ctx context.Context, workflowId int, status, keyword string, page, pageSize int) (int, []Run, error) {
	m := g.DB().Model("bm_workflow_runs").Ctx(ctx).Where("workflow_id", workflowId)
	if status != "" {
		m = m.Where("status", status)
	}
	if keyword != "" {
		m = m.WhereLike("email", "%"+keyword+"%")
	}
	total, err := m.Count()
	if err != nil {
		return 0, nil, err
	}
	var list []Run
	err = m.OrderDesc("id").Page(page, pageSize).Scan(&list)
	if list == nil {
		list = []Run{}
	}
	return total, list, err
}

// LogEntry is one step result for one contact.
type LogEntry struct {
	Id         int64  `json:"id" orm:"id"`
	WorkflowId int    `json:"workflow_id" orm:"workflow_id"`
	RunId      int64  `json:"run_id" orm:"run_id"`
	ContactId  int    `json:"contact_id" orm:"contact_id"`
	Email      string `json:"email" orm:"email"`
	StepIndex  int    `json:"step_index" orm:"step_index"`
	StepPath   string `json:"step_path" orm:"step_path"`
	StepType   string `json:"step_type" orm:"step_type"`
	Status     string `json:"status" orm:"status"`
	Detail     string `json:"detail" orm:"detail"`
	MessageId  string `json:"message_id" orm:"message_id"`
	CreateTime int    `json:"create_time" orm:"create_time"`
}

// ListLogs pages through what the workflow did.
func ListLogs(ctx context.Context, workflowId int, status string, page, pageSize int) (int, []LogEntry, error) {
	m := g.DB().Model("bm_workflow_logs").Ctx(ctx).Where("workflow_id", workflowId)
	if status != "" {
		m = m.Where("status", status)
	}
	total, err := m.Count()
	if err != nil {
		return 0, nil, err
	}
	var list []LogEntry
	err = m.OrderDesc("id").Page(page, pageSize).Scan(&list)
	if list == nil {
		list = []LogEntry{}
	}
	return total, list, err
}
