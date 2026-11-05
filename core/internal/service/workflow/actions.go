package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"billionmail-core/internal/model/entity"
	"billionmail-core/internal/service/batch_mail"
	"billionmail-core/internal/service/domains"
	"billionmail-core/internal/service/mail_service"
	"billionmail-core/internal/service/maillog_stat"
	"billionmail-core/internal/service/public"
	"billionmail-core/internal/service/sms"
	"billionmail-core/internal/service/whatsapp"

	"github.com/gogf/gf/v2/frame/g"
)

// CampaignBase offsets workflow ids in open/click tracking so they never collide with
// campaign task ids (API templates use 1,000,000,000+).
const CampaignBase = 1500000000

// runAction performs a step that does something (not wait/condition/split) and returns
// a log status, a readable detail and, for emails, the message id used for tracking.
func runAction(ctx context.Context, w *Workflow, step Step, contact *entity.Contact) (string, string, string) {
	switch step.Type {
	case StepEmail:
		if contact.Active != 1 {
			return LogSkipped, "contact is unsubscribed", ""
		}
		messageId, err := sendEmail(ctx, w.Id, step, *contact)
		if err != nil {
			return LogFailed, "email failed: " + err.Error(), ""
		}
		return LogDone, "email sent: " + step.Subject, messageId

	case StepSMS:
		phone := strings.TrimSpace(contact.Attribs[sms.AttrPhone])
		if contact.Active != 1 {
			return LogSkipped, "contact is unsubscribed", ""
		}
		if phone == "" {
			return LogSkipped, "contact has no phone number", ""
		}
		if err := sms.SendOne(ctx, phone, sms.Personalize(step.Message, *contact), contact.GroupId); err != nil {
			return LogFailed, "SMS failed: " + err.Error(), ""
		}
		return LogDone, "SMS sent to " + phone, ""

	case StepWhatsApp:
		phone := strings.TrimSpace(contact.Attribs[sms.AttrPhone])
		if contact.Active != 1 {
			return LogSkipped, "contact is unsubscribed", ""
		}
		if phone == "" {
			return LogSkipped, "contact has no phone number", ""
		}
		tpl := whatsapp.Template{Name: step.WaTemplate, Language: step.WaLanguage}
		for _, p := range step.WaParams {
			tpl.Params = append(tpl.Params, sms.Personalize(p, *contact))
		}
		if err := whatsapp.SendOne(ctx, phone, tpl); err != nil {
			return LogFailed, "WhatsApp failed: " + err.Error(), ""
		}
		return LogDone, "WhatsApp " + step.WaTemplate + " sent to " + phone, ""

	case StepNotify:
		return notifyStaff(ctx, w, step, *contact)

	case StepAddTag:
		_, err := g.DB().Exec(ctx, `
			INSERT INTO bm_contact_tags (contact_id, tag_id, create_time) VALUES (?, ?, ?)
			ON CONFLICT (contact_id, tag_id) DO NOTHING`,
			contact.Id, step.TagId, time.Now().Unix())
		if err != nil {
			return LogFailed, "add tag failed: " + err.Error(), ""
		}
		return LogDone, "tag added: " + tagName(ctx, step.TagId), ""

	case StepRemoveTag:
		_, err := g.DB().Model("bm_contact_tags").Ctx(ctx).
			Where("contact_id", contact.Id).Where("tag_id", step.TagId).Delete()
		if err != nil {
			return LogFailed, "remove tag failed: " + err.Error(), ""
		}
		return LogDone, "tag removed: " + tagName(ctx, step.TagId), ""

	case StepSetField:
		value := ExpandValue(step.Value, *contact, time.Now())
		_, err := g.DB().Exec(ctx, `
			UPDATE bm_contacts SET attribs = COALESCE(attribs, '{}'::jsonb) || jsonb_build_object(?::text, ?::text)
			WHERE id = ?`, step.Attribute, value, contact.Id)
		if err != nil {
			return LogFailed, "update field failed: " + err.Error(), ""
		}
		if contact.Attribs == nil {
			contact.Attribs = map[string]string{}
		}
		contact.Attribs[step.Attribute] = value
		return LogDone, fmt.Sprintf("%s set to %q", step.Attribute, value), ""

	case StepDelField:
		// Rebuild the object without the key; "jsonb - ?" clashes with the driver's ? placeholders.
		_, err := g.DB().Exec(ctx, `
			UPDATE bm_contacts SET attribs = (
				SELECT COALESCE(jsonb_object_agg(key, value), '{}'::jsonb)
				FROM jsonb_each(COALESCE(attribs, '{}'::jsonb)) WHERE key <> ?
			) WHERE id = ?`,
			step.Attribute, contact.Id)
		if err != nil {
			return LogFailed, "remove field failed: " + err.Error(), ""
		}
		delete(contact.Attribs, step.Attribute)
		return LogDone, step.Attribute + " removed", ""

	case StepAddGroup:
		return addToGroup(ctx, step.GroupId, *contact)

	case StepWebhook:
		return callWebhook(ctx, w, step, *contact)
	}
	return LogSkipped, "unknown step " + step.Type, ""
}

// addToGroup copies the contact into another group (same email and fields). Adding them
// can start workflows that trigger on that group.
func addToGroup(ctx context.Context, groupId int, contact entity.Contact) (string, string, string) {
	name, _ := g.DB().Model("bm_contact_groups").Ctx(ctx).Where("id", groupId).Value("name")
	if name.IsEmpty() {
		return LogFailed, fmt.Sprintf("group #%d no longer exists", groupId), ""
	}
	n, err := g.DB().Model("bm_contacts").Ctx(ctx).Where("group_id", groupId).Where("email", contact.Email).Count()
	if err != nil {
		return LogFailed, "add to group failed: " + err.Error(), ""
	}
	if n > 0 {
		return LogDone, "already in group " + name.String(), ""
	}
	attribs, _ := json.Marshal(contact.Attribs)
	if contact.Attribs == nil {
		attribs = []byte("{}")
	}
	_, err = g.DB().Exec(ctx, `
		INSERT INTO bm_contacts (email, group_id, active, task_id, create_time, status, attribs, last_active_at)
		VALUES (?, ?, ?, 0, ?, 1, ?::jsonb, 0)`,
		contact.Email, groupId, contact.Active, time.Now().Unix(), string(attribs))
	if err != nil {
		return LogFailed, "add to group failed: " + err.Error(), ""
	}
	return LogDone, "added to group " + name.String(), ""
}

// webhookClient has a short timeout so a slow endpoint can't hold up the workflow timer.
var webhookClient = &http.Client{Timeout: 10 * time.Second}

// WebhookPayload is the JSON a webhook step posts.
type WebhookPayload struct {
	Event    string            `json:"event"`
	Workflow map[string]any    `json:"workflow"`
	Contact  map[string]any    `json:"contact"`
	Fields   map[string]string `json:"fields"`
	SentAt   string            `json:"sent_at"`
}

// callWebhook POSTs the contact as JSON to the step's URL. 2xx counts as done.
func callWebhook(ctx context.Context, w *Workflow, step Step, contact entity.Contact) (string, string, string) {
	headers, err := ParseHeaders(step.WebhookHeaders)
	if err != nil {
		return LogFailed, err.Error(), ""
	}
	fields := contact.Attribs
	if fields == nil {
		fields = map[string]string{}
	}
	body, _ := json.Marshal(WebhookPayload{
		Event:    "workflow.step",
		Workflow: map[string]any{"id": w.Id, "name": w.Name},
		Contact:  map[string]any{"id": contact.Id, "email": contact.Email, "group_id": contact.GroupId, "subscribed": contact.Active == 1},
		Fields:   fields,
		SentAt:   time.Now().UTC().Format(time.RFC3339),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(step.WebhookURL), bytes.NewReader(body))
	if err != nil {
		return LogFailed, "webhook failed: " + err.Error(), ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "BillionMail-Workflow/1.0")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := webhookClient.Do(req)
	if err != nil {
		return LogFailed, "webhook failed: " + err.Error(), ""
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return LogFailed, fmt.Sprintf("webhook failed: %s answered %d", req.URL.Host, resp.StatusCode), ""
	}
	return LogDone, fmt.Sprintf("webhook sent to %s (%d)", req.URL.Host, resp.StatusCode), ""
}

var todayRe = regexp.MustCompile(`\{today([+-]\d+)?\}`)

// ExpandValue fills {today}, {today+7}, {today-1} (YYYY-MM-DD) and contact placeholders like {name}.
func ExpandValue(v string, contact entity.Contact, now time.Time) string {
	v = todayRe.ReplaceAllStringFunc(v, func(m string) string {
		days := 0
		if sub := todayRe.FindStringSubmatch(m); len(sub) > 1 && sub[1] != "" {
			days, _ = strconv.Atoi(sub[1])
		}
		return now.AddDate(0, 0, days).Format("2006-01-02")
	})
	return sms.Personalize(v, contact)
}

// notifyStaff alerts clinic staff about this contact by email or SMS.
func notifyStaff(ctx context.Context, w *Workflow, step Step, contact entity.Contact) (string, string, string) {
	text := sms.Personalize(step.Message, contact)
	to := SplitList(step.NotifyTo)
	var failed []string
	switch step.NotifyChannel {
	case NotifyEmail:
		sender, err := mail_service.NewEmailSenderWithLocal(step.Addresser)
		if err != nil {
			return LogFailed, "staff alert failed: " + err.Error(), ""
		}
		defer sender.Close()
		subject := step.Subject
		if strings.TrimSpace(subject) == "" {
			subject = "Workflow alert: " + w.Name
		}
		body := fmt.Sprintf(`<p>%s</p><hr><p style="color:#888;font-size:12px">Contact: %s<br>Phone: %s<br>Workflow: %s</p>`,
			strings.ReplaceAll(htmlEscape(text), "\n", "<br>"), htmlEscape(contact.Email),
			htmlEscape(contact.Attribs[sms.AttrPhone]), htmlEscape(w.Name))
		message := mail_service.NewMessage(sms.Personalize(subject, contact), body)
		message.SetMessageID(sender.GenerateMessageID())
		if step.FullName != "" {
			message.SetRealName(step.FullName)
		}
		if err := sender.Send(message, to); err != nil {
			return LogFailed, "staff alert failed: " + err.Error(), ""
		}
	case NotifySMS:
		for _, phone := range to {
			if err := sms.SendOne(ctx, phone, text, 0); err != nil {
				failed = append(failed, phone+": "+err.Error())
			}
		}
		if len(failed) == len(to) {
			return LogFailed, "staff alert failed: " + strings.Join(failed, "; "), ""
		}
	}
	detail := "staff alerted: " + strings.Join(to, ", ")
	if len(failed) > 0 {
		detail += " (failed: " + strings.Join(failed, "; ") + ")"
	}
	return LogDone, detail, ""
}

func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// checkCondition reports whether the condition is true for the contact, with a readable reason.
func checkCondition(ctx context.Context, c Cond, contact entity.Contact, run *Run) (bool, string) {
	switch c.Condition {
	case CondHasTag, CondNotHasTag:
		n, err := g.DB().Model("bm_contact_tags").Ctx(ctx).
			Where("contact_id", contact.Id).Where("tag_id", c.TagId).Count()
		if err != nil {
			return false, "could not check tag: " + err.Error()
		}
		name := tagName(ctx, c.TagId)
		has := n > 0
		reason := "does not have tag " + name
		if has {
			reason = "has tag " + name
		}
		if c.Condition == CondHasTag {
			return has, reason
		}
		return !has, reason
	case CondHasPhone:
		if strings.TrimSpace(contact.Attribs[sms.AttrPhone]) != "" {
			return true, "has a phone number"
		}
		return false, "no phone number"
	case CondSubscribed:
		if contact.Active == 1 {
			return true, "is subscribed"
		}
		return false, "unsubscribed"
	case CondAttrEquals:
		return attrEquals(contact.Attribs, c.Attribute, c.Value)
	case CondAttrSet:
		if v := strings.TrimSpace(contact.Attribs[c.Attribute]); v != "" {
			return true, fmt.Sprintf("%s is %q", c.Attribute, v)
		}
		return false, c.Attribute + " is empty"
	case CondOpenedCampaign, CondClickedCampaign:
		table, verb := "mailstat_opened", "opened"
		if c.Condition == CondClickedCampaign {
			table, verb = "mailstat_clicked", "clicked"
		}
		n, err := g.DB().Model(table).Ctx(ctx).Where("campaign_id", c.TaskId).Where("recipient", contact.Email).Count()
		if err != nil {
			return false, "could not check campaign: " + err.Error()
		}
		if n > 0 {
			return true, fmt.Sprintf("%s campaign #%d", verb, c.TaskId)
		}
		return false, fmt.Sprintf("has not %s campaign #%d", verb, c.TaskId)
	case CondOpenedLastEmail, CondClickedLastEmail:
		if run == nil {
			return false, "no earlier email in this workflow"
		}
		table, verb := "mailstat_opened", "opened"
		if c.Condition == CondClickedLastEmail {
			table, verb = "mailstat_clicked", "clicked"
		}
		v, err := g.DB().Model("bm_workflow_logs").Ctx(ctx).
			Where("run_id", run.Id).WhereNot("message_id", "").OrderDesc("id").Value("message_id")
		if err != nil || v.IsEmpty() {
			return false, "no earlier email in this workflow"
		}
		n, err := g.DB().Model(table).Ctx(ctx).Where("message_id", v.String()).Count()
		if err != nil {
			return false, "could not check email: " + err.Error()
		}
		if n > 0 {
			return true, verb + " the last email"
		}
		return false, "has not " + verb + " the last email"
	}
	return false, "unknown condition " + c.Condition
}

// attrEquals compares a contact attribute, ignoring case and surrounding spaces.
func attrEquals(attribs map[string]string, attr, value string) (bool, string) {
	got := strings.TrimSpace(attribs[attr])
	if strings.EqualFold(got, strings.TrimSpace(value)) {
		return true, fmt.Sprintf("%s is %q", attr, value)
	}
	return false, fmt.Sprintf("%s is %q, not %q", attr, got, value)
}

func tagName(ctx context.Context, tagId int) string {
	v, err := g.DB().Model("bm_tags").Ctx(ctx).Where("id", tagId).Value("name")
	if err != nil || v.IsEmpty() {
		return fmt.Sprintf("#%d", tagId)
	}
	return v.String()
}

// sendEmail renders the template for the contact and sends it from the chosen mailbox,
// with open/click tracking and an unsubscribe link for the contact's group.
// It returns the message id (without brackets) that opens and clicks are recorded under.
func sendEmail(ctx context.Context, workflowId int, step Step, contact entity.Contact) (string, error) {
	var tpl entity.EmailTemplate
	if err := g.DB().Model("email_templates").Ctx(ctx).Where("id", step.TemplateId).Scan(&tpl); err != nil {
		return "", err
	}
	if tpl.Id == 0 {
		return "", fmt.Errorf("email template %d not found", step.TemplateId)
	}

	baseURL := domains.GetBaseURLBySender(step.Addresser)
	content := tpl.Content
	if !strings.Contains(content, "__UNSUBSCRIBE_URL__") && !strings.Contains(content, "{{ UnsubscribeURL . }}") {
		content = public.AddUnsubscribeButton(content)
	}
	unsubscribeURL := ""
	if contact.GroupId > 0 {
		token, err := batch_mail.GenerateUnsubscribeJWT(contact.Email, tpl.Id, 0, contact.GroupId)
		if err == nil {
			unsubscribeURL = fmt.Sprintf("%s/unsubscribe_new.html?jwt=%s", baseURL, token)
		}
	}

	engine := batch_mail.GetTemplateEngine()
	if rendered, err := engine.RenderEmailTemplate(ctx, content, &contact, nil, unsubscribeURL); err == nil {
		content = rendered
	}
	subject := step.Subject
	if rendered, err := engine.RenderEmailTemplate(ctx, subject, &contact, nil, unsubscribeURL); err == nil {
		subject = rendered
	}
	content = strings.ReplaceAll(content, "__UNSUBSCRIBE_URL__", unsubscribeURL)

	sender, err := mail_service.NewEmailSenderWithLocal(step.Addresser)
	if err != nil {
		return "", err
	}
	defer sender.Close()

	messageId := sender.GenerateMessageID()
	tracker := maillog_stat.NewMailTracker(content, CampaignBase+workflowId, messageId, contact.Email, baseURL)
	tracker.TrackLinks()
	tracker.AppendTrackingPixel()
	content = tracker.GetHTML()

	message := mail_service.NewMessage(subject, content)
	message.SetMessageID(messageId)
	if step.FullName != "" {
		message.SetRealName(step.FullName)
	}
	if err := sender.Send(message, []string{contact.Email}); err != nil {
		return "", err
	}
	return strings.Trim(messageId, "<>"), nil
}
