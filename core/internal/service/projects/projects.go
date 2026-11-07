package projects

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

var ErrConflict = errors.New("project revision or idempotency key conflicts with an existing change")
var ErrUnavailable = errors.New("project is unavailable in this workspace")
var languageTag = regexp.MustCompile(`^[A-Za-z]{2,8}(-[A-Za-z0-9]{1,8})*$`)

type Requirements struct {
	Audience      string   `json:"audience"`
	Channels      []string `json:"channels"`
	Languages     []string `json:"languages"`
	SuccessMetric string   `json:"success_metric"`
	Constraints   string   `json:"constraints"`
}
type Input struct {
	ID           string       `json:"id"`
	Revision     int64        `json:"revision"`
	Title        string       `json:"title"`
	Objective    string       `json:"objective"`
	Requirements Requirements `json:"requirements"`
}
type Project struct {
	Input
	Status    string    `json:"status"`
	Missing   []string  `json:"missing"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
type Event struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}
type EventsPage struct {
	Items     []Event `json:"items"`
	NextAfter int64   `json:"next_after"`
}

type Page struct {
	Items      []Project `json:"items"`
	NextCursor string    `json:"next_cursor"`
}

func List(ctx context.Context, tenant tenancy.TenantContext, cursor string, limit int) (Page, error) {
	if err := tenancy.Authorize(tenant, "project.read"); err != nil {
		return Page{}, err
	}
	if limit < 1 || limit > 100 {
		return Page{}, errors.New("invalid project page size")
	}
	var before time.Time
	var beforeID uuid.UUID
	if cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return Page{}, errors.New("invalid project cursor")
		}
		parts := strings.Split(string(data), "|")
		if len(parts) != 2 {
			return Page{}, errors.New("invalid project cursor")
		}
		before, err = time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return Page{}, errors.New("invalid project cursor")
		}
		beforeID, err = uuid.Parse(parts[1])
		if err != nil {
			return Page{}, errors.New("invalid project cursor")
		}
	}
	page := Page{Items: []Project{}}
	err := transaction(ctx, tenant, func(tx gdb.TX) error {
		var rows []struct {
			ID        string
			CreatedAt time.Time
		}
		query := tx.Model("marketing_projects").Fields("id,created_at").Where("company_id", tenant.CompanyID).WhereNull("deleted_at")
		if cursor != "" {
			query = query.Where("(created_at,id)<(?,?)", before, beforeID)
		}
		if err := query.Order("created_at DESC,id DESC").Limit(limit + 1).Scan(&rows); err != nil {
			return err
		}
		for index, row := range rows {
			if index == limit {
				last := rows[index-1]
				page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID))
				break
			}
			project, err := get(tx, tenant.CompanyID, row.ID, false)
			if err != nil {
				return err
			}
			page.Items = append(page.Items, project)
		}
		return nil
	})
	return page, err
}

// Missing is deterministic discovery guidance, never an invented AI response.
func Missing(r Requirements) []string {
	fields := []string{}
	if strings.TrimSpace(r.Audience) == "" {
		fields = append(fields, "audience")
	}
	if len(r.Channels) == 0 {
		fields = append(fields, "channels")
	}
	if len(r.Languages) == 0 {
		fields = append(fields, "languages")
	}
	if strings.TrimSpace(r.SuccessMetric) == "" {
		fields = append(fields, "success_metric")
	}
	return fields
}
func Validate(input Input) error {
	if strings.TrimSpace(input.Title) == "" || len(input.Title) > 200 || strings.TrimSpace(input.Objective) == "" || len(input.Objective) > 16000 || input.Revision < 0 {
		return errors.New("bounded project title, objective and nonnegative revision are required")
	}
	if input.ID != "" {
		if id, err := uuid.Parse(input.ID); err != nil || id == uuid.Nil {
			return errors.New("invalid project ID")
		}
	}
	r := input.Requirements
	if len(r.Audience) > 8000 || len(r.SuccessMetric) > 2000 || len(r.Constraints) > 8000 || len(r.Channels) > 8 || len(r.Languages) > 20 {
		return errors.New("project requirements exceed field limits")
	}
	allowed := map[string]bool{"email": true, "whatsapp": true, "sms": true, "push": true, "social": true, "blog": true, "seo": true, "landing_page": true}
	seen := map[string]bool{}
	for _, channel := range r.Channels {
		if !allowed[channel] || seen[channel] {
			return errors.New("channels must be supported and unique")
		}
		seen[channel] = true
	}
	seen = map[string]bool{}
	for _, language := range r.Languages {
		key := strings.ToLower(language)
		if len(language) > 32 || !languageTag.MatchString(language) || seen[key] {
			return errors.New("languages must be unique language tags")
		}
		seen[key] = true
	}
	return nil
}
func transaction(ctx context.Context, tenant tenancy.TenantContext, fn func(gdb.TX) error) error {
	if tenant.CompanyID == uuid.Nil {
		return errors.New("workspace is required")
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		return fn(tx)
	})
}
func get(tx gdb.TX, company uuid.UUID, id string, lock bool) (Project, error) {
	var row struct {
		ID                       string
		Revision                 int64
		Title, Objective, Status string
		Requirements             json.RawMessage
		CreatedAt, UpdatedAt     time.Time
	}
	model := tx.Model("marketing_projects").Where("company_id", company).Where("id", id).WhereNull("deleted_at")
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrUnavailable
		}
		return Project{}, err
	}
	if row.ID == "" {
		return Project{}, ErrUnavailable
	}
	r := Requirements{}
	if err := json.Unmarshal(row.Requirements, &r); err != nil {
		return Project{}, err
	}
	if r.Channels == nil {
		r.Channels = []string{}
	}
	if r.Languages == nil {
		r.Languages = []string{}
	}
	return Project{Input: Input{ID: row.ID, Revision: row.Revision, Title: row.Title, Objective: row.Objective, Requirements: r}, Status: row.Status, Missing: Missing(r), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}
func Get(ctx context.Context, tenant tenancy.TenantContext, id string) (Project, error) {
	if err := tenancy.Authorize(tenant, "project.read"); err != nil {
		return Project{}, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil || parsed == uuid.Nil {
		return Project{}, ErrUnavailable
	}
	id = parsed.String()
	var result Project
	err = transaction(ctx, tenant, func(tx gdb.TX) error { var err error; result, err = get(tx, tenant.CompanyID, id, false); return err })
	return result, err
}

// mutate serializes commands and stores their exact response with the state and
// event. Retried commands therefore neither duplicate history nor regress state.
func mutate(ctx context.Context, tenant tenancy.TenantContext, key, operation string, input any, result any, fn func(gdb.TX) error) error {
	if err := tenancy.Authorize(tenant, "project.manage"); err != nil {
		return err
	}
	if tenant.UserID == uuid.Nil || key == "" || len(key) > 128 {
		return errors.New("actor and idempotency key are required")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	hash := hex.EncodeToString(digest[:])
	return transaction(ctx, tenant, func(tx gdb.TX) error {
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", tenant.CompanyID.String()+":projects"); err != nil {
			return err
		}
		var receipt struct {
			RequestHash string
			Response    json.RawMessage
		}
		if err := tx.Model("marketing_mutation_receipts").Where("company_id", tenant.CompanyID).Where("operation", operation).Where("idempotency_key", key).Scan(&receipt); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if receipt.RequestHash != "" {
			if receipt.RequestHash != hash {
				return ErrConflict
			}
			return json.Unmarshal(receipt.Response, result)
		}
		if err := fn(tx); err != nil {
			return err
		}
		response, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err = tx.Model("marketing_mutation_receipts").Data(g.Map{"company_id": tenant.CompanyID, "operation": operation, "idempotency_key": key, "request_hash": hash, "response": string(response), "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		resourceType, resourceID := "marketing_project", ""
		switch value := result.(type) {
		case *Project:
			resourceID = value.ID
		case *Message:
			resourceType, resourceID = "marketing_project_message", value.ID
		}
		_, err = tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": operation, "resource_type": resourceType, "resource_id": resourceID, "metadata": g.Map{"source": "user"}}).Insert()
		return err
	})
}
func event(tx gdb.TX, tenant tenancy.TenantContext, id, kind string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Model("marketing_project_events").Data(g.Map{"company_id": tenant.CompanyID, "project_id": id, "type": kind, "payload": string(data), "created_by": tenant.UserID}).Insert()
	return err
}
func Save(ctx context.Context, tenant tenancy.TenantContext, key string, input Input) (Project, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Objective = strings.TrimSpace(input.Objective)
	if input.Requirements.Channels == nil {
		input.Requirements.Channels = []string{}
	}
	if input.Requirements.Languages == nil {
		input.Requirements.Languages = []string{}
	}
	if err := Validate(input); err != nil {
		return Project{}, err
	}
	if input.ID != "" {
		parsed, _ := uuid.Parse(input.ID)
		input.ID = parsed.String()
	}
	var result Project
	err := mutate(ctx, tenant, key, "project.save", input, &result, func(tx gdb.TX) error {
		now := time.Now().UTC()
		r, _ := json.Marshal(input.Requirements)
		if input.ID == "" {
			if input.Revision != 0 {
				return ErrConflict
			}
			result = Project{Input: input, Status: "discovery", Missing: Missing(input.Requirements), CreatedAt: now, UpdatedAt: now}
			result.ID = uuid.NewString()
			result.Revision = 1
			_, err := tx.Model("marketing_projects").Data(g.Map{"id": result.ID, "company_id": tenant.CompanyID, "title": input.Title, "objective": input.Objective, "requirements": string(r), "created_by": tenant.UserID, "updated_by": tenant.UserID, "created_at": now, "updated_at": now}).Insert()
			if err != nil {
				return err
			}
		} else {
			current, err := get(tx, tenant.CompanyID, input.ID, true)
			if err != nil {
				return err
			}
			if current.Revision != input.Revision {
				return ErrConflict
			}
			// Later stages must supersede strategy/approval versions explicitly.
			if current.Status != "discovery" {
				return ErrConflict
			}
			result = current
			result.Input = input
			result.Revision++
			result.UpdatedAt = now
			result.Missing = Missing(input.Requirements)
			if _, err = tx.Model("marketing_projects").Where("company_id", tenant.CompanyID).Where("id", input.ID).Data(g.Map{"title": input.Title, "objective": input.Objective, "requirements": string(r), "revision": result.Revision, "updated_at": now, "updated_by": tenant.UserID}).Update(); err != nil {
				return err
			}
		}
		return event(tx, tenant, result.ID, "project.updated", result)
	})
	return result, err
}
func AddMessage(ctx context.Context, tenant tenancy.TenantContext, key, id, content string) (Message, error) {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed == uuid.Nil {
		return Message{}, ErrUnavailable
	}
	id = parsed.String()
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 16000 {
		return Message{}, errors.New("message must contain 1 to 16000 bytes")
	}
	var result Message
	err = mutate(ctx, tenant, key, "project.message", g.Map{"project_id": id, "content": content}, &result, func(tx gdb.TX) error {
		if _, err := get(tx, tenant.CompanyID, id, true); err != nil {
			return err
		}
		result = Message{ID: uuid.NewString(), Role: "user", Content: content, CreatedAt: time.Now().UTC()}
		if _, err := tx.Model("marketing_project_messages").Data(g.Map{"id": result.ID, "company_id": tenant.CompanyID, "project_id": id, "role": result.Role, "content": content, "created_at": result.CreatedAt, "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		return event(tx, tenant, id, "message.created", result)
	})
	return result, err
}
func Events(ctx context.Context, tenant tenancy.TenantContext, id string, after int64, limit int) (EventsPage, error) {
	project, err := Get(ctx, tenant, id)
	if err != nil {
		return EventsPage{}, err
	}
	id = project.ID
	if after < 0 || limit < 1 || limit > 100 {
		return EventsPage{}, errors.New("invalid event cursor or limit")
	}
	page := EventsPage{Items: []Event{}, NextAfter: after}
	err = transaction(ctx, tenant, func(tx gdb.TX) error {
		if err := tx.Model("marketing_project_events").Where("company_id", tenant.CompanyID).Where("project_id", id).Where("id > ?", after).WhereNull("deleted_at").OrderAsc("id").Limit(limit).Scan(&page.Items); err != nil {
			return err
		}
		if len(page.Items) > 0 {
			page.NextAfter = page.Items[len(page.Items)-1].ID
		}
		return nil
	})
	return page, err
}
