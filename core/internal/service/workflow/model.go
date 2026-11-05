// Package workflow runs Klaviyo-style automation flows: a trigger enrolls contacts,
// then each contact walks through steps (wait, email, SMS, WhatsApp, staff alerts,
// tag and field changes), with Yes/No and A/B branches, exit conditions, quiet hours
// and smart sending.
package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // quiet hours need time zones even on minimal images
)

// Workflow statuses.
const (
	StatusDraft  = "draft"
	StatusActive = "active"
	StatusPaused = "paused"
)

// Run statuses.
const (
	RunActive    = "active"
	RunCompleted = "completed"
	RunExited    = "exited"
	RunSkipped   = "skipped" // never started: failed flow filters or re-entry rule
)

// Log statuses.
const (
	LogDone    = "done"
	LogSkipped = "skipped"
	LogFailed  = "failed"
	LogExited  = "exited"
	LogWaiting = "waiting" // held back by quiet hours
	LogReview  = "review"  // manual message waiting for someone to send it
)

// Trigger types.
const (
	TriggerGroupJoin    = "group_join"    // contact added to a group
	TriggerTagAdded     = "tag_added"     // tag added to a contact
	TriggerDate         = "date"          // N days before/after a date attribute (e.g. last_visit, birthday)
	TriggerEmailOpened  = "email_opened"  // opened a campaign email
	TriggerEmailClicked = "email_clicked" // clicked a link in a campaign email
)

// Step types.
const (
	StepWait      = "wait"
	StepEmail     = "email"
	StepSMS       = "sms"
	StepWhatsApp  = "whatsapp"
	StepNotify    = "notify" // alert clinic staff
	StepAddTag    = "add_tag"
	StepRemoveTag = "remove_tag"
	StepSetField  = "set_attribute"
	StepCondition = "condition" // continue only if
	StepSplit     = "split"     // Yes/No branch or A/B test
	StepWebhook   = "webhook"   // send contact data to another system (e.g. n8n)
	StepAddGroup  = "add_to_group"
	StepDelField  = "remove_attribute"
)

// Message statuses (Klaviyo-style): live sends automatically, manual queues the
// message for review, draft skips it.
const (
	MessageLive   = "live"
	MessageManual = "manual"
	MessageDraft  = "draft"
)

// Re-entry rules: can someone go through the workflow more than once?
const (
	ReentryAlways = "always" // every time they qualify again (default)
	ReentryOnce   = "once"   // only ever once
	ReentryAfter  = "after"  // again only after ReentryDays since they last entered
)

// Split modes.
const (
	SplitCondition = "condition"
	SplitRandom    = "random"
)

// Condition kinds, used by condition steps, splits and exit conditions.
const (
	CondHasTag           = "has_tag"
	CondNotHasTag        = "not_has_tag"
	CondHasPhone         = "has_phone"
	CondSubscribed       = "subscribed"
	CondAttrEquals       = "attr_equals"
	CondAttrSet          = "attr_set"
	CondOpenedCampaign   = "opened_campaign"
	CondClickedCampaign  = "clicked_campaign"
	CondOpenedLastEmail  = "opened_last_email"  // opened the previous email of this workflow
	CondClickedLastEmail = "clicked_last_email" // clicked the previous email of this workflow
)

// Notify channels.
const (
	NotifyEmail = "email"
	NotifySMS   = "sms"
)

// Limits keep flows reasonable and bound the work done per contact per tick.
const (
	MaxSteps       = 100 // across all branches
	MaxBranchDepth = 3
)

// Trigger decides who enters a workflow.
type Trigger struct {
	Type    string `json:"type"`
	GroupId int    `json:"group_id"` // group_join: required; date: optional (0 = all groups)
	TagId   int    `json:"tag_id"`   // tag_added
	TaskId  int    `json:"task_id"`  // email_opened, email_clicked: the campaign
	// Date trigger: contact attribute holding a date (YYYY-MM-DD, DD/MM/YYYY, ...).
	Attribute  string `json:"attribute"`
	OffsetDays int    `json:"offset_days"` // days after the date (negative = before)
	Yearly     bool   `json:"yearly"`      // repeat every year on the same day (birthdays)
	Monthly    bool   `json:"monthly"`     // repeat every month on the same day
	// Reentry: always (default), once, or after ReentryDays.
	Reentry     string `json:"reentry"`
	ReentryDays int    `json:"reentry_days"`
	// IncludeExisting enrolls contacts already in the group/tag/opens when the workflow is activated.
	IncludeExisting bool `json:"include_existing"`
}

// Cond is a check on a contact.
type Cond struct {
	Condition string `json:"condition,omitempty"`
	TagId     int    `json:"tag_id,omitempty"`
	Attribute string `json:"attribute,omitempty"`
	Value     string `json:"value,omitempty"`
	TaskId    int    `json:"task_id,omitempty"`
}

// Step is one box in a workflow. Only the fields for its Type are used.
type Step struct {
	Type string `json:"type"`

	// condition, split (condition mode), add_tag/remove_tag (tag_id), set_attribute (attribute, value)
	Cond

	// wait
	WaitValue int    `json:"wait_value,omitempty"`
	WaitUnit  string `json:"wait_unit,omitempty"` // minutes, hours, days
	// Days only: send at this time of day ("HH:MM") and/or only on these weekdays (0=Sun..6=Sat).
	AtTime   string `json:"at_time,omitempty"`
	Weekdays []int  `json:"weekdays,omitempty"`
	Timezone string `json:"timezone,omitempty"`

	// email, sms, whatsapp: Klaviyo-style message settings
	Status           string `json:"status,omitempty"`             // live (default), manual, draft
	Filters          []Cond `json:"filters,omitempty"`            // all must be true, otherwise the message is skipped
	SkipSmartSending bool   `json:"skip_smart_sending,omitempty"` // send even if the contact was messaged recently

	// webhook
	WebhookURL     string `json:"webhook_url,omitempty"`
	WebhookHeaders string `json:"webhook_headers,omitempty"` // "Name: value" per line

	// add_to_group
	GroupId int `json:"group_id,omitempty"`

	// email, notify by email
	TemplateId int    `json:"template_id,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Addresser  string `json:"addresser,omitempty"`
	FullName   string `json:"full_name,omitempty"`

	// sms, notify
	Message string `json:"message,omitempty"`

	// whatsapp
	WaTemplate string   `json:"wa_template,omitempty"`
	WaLanguage string   `json:"wa_language,omitempty"`
	WaParams   []string `json:"wa_params,omitempty"`

	// notify
	NotifyChannel string `json:"notify_channel,omitempty"`
	NotifyTo      string `json:"notify_to,omitempty"` // emails or phone numbers, comma separated

	// split
	SplitMode string `json:"split_mode,omitempty"` // condition, random
	Percent   int    `json:"percent,omitempty"`    // random: share of contacts going to Yes (A)
	Yes       []Step `json:"yes,omitempty"`
	No        []Step `json:"no,omitempty"`
}

// QuietHours holds back messages outside a daily sending window.
type QuietHours struct {
	Enabled  bool   `json:"enabled"`
	Start    int    `json:"start"`    // first hour messages may go out, 0-23
	End      int    `json:"end"`      // hour sending stops, 0-23 (may be before Start to span midnight)
	Timezone string `json:"timezone"` // e.g. Asia/Dubai
}

// Settings apply to the whole workflow.
type Settings struct {
	QuietHours QuietHours `json:"quiet_hours"`
	// SmartSendingHours skips a message when the contact got any workflow message in the last N hours (0 = off).
	SmartSendingHours int `json:"smart_sending_hours"`
	// ExitConditions take a contact out as soon as any one is true (e.g. booked an appointment).
	ExitConditions []Cond `json:"exit_conditions"`
	// FlowFilters (Klaviyo profile filters) must all be true: checked when someone enters
	// (no entry if they fail) and before every message (the message is skipped if they fail).
	FlowFilters []Cond `json:"flow_filters"`
}

// IsMessage reports whether the step sends something to the contact.
func (s Step) IsMessage() bool {
	return s.Type == StepEmail || s.Type == StepSMS || s.Type == StepWhatsApp
}

// Validate checks the trigger has what its type needs.
func (t Trigger) Validate() error {
	switch t.Type {
	case TriggerGroupJoin:
		if t.GroupId <= 0 {
			return errors.New("choose the contact group that starts this workflow")
		}
	case TriggerTagAdded:
		if t.TagId <= 0 {
			return errors.New("choose the tag that starts this workflow")
		}
	case TriggerDate:
		if strings.TrimSpace(t.Attribute) == "" {
			return errors.New("enter the contact attribute that holds the date, e.g. last_visit")
		}
		if t.OffsetDays < -3650 || t.OffsetDays > 3650 {
			return errors.New("days before/after must be within 10 years")
		}
	case TriggerEmailOpened, TriggerEmailClicked:
		if t.TaskId <= 0 {
			return errors.New("choose the email campaign")
		}
	default:
		return fmt.Errorf("unknown trigger type %q", t.Type)
	}
	switch t.Reentry {
	case "", ReentryAlways, ReentryOnce:
	case ReentryAfter:
		if t.ReentryDays < 1 || t.ReentryDays > 3650 {
			return errors.New("re-entry: choose after how many days (1-3650)")
		}
	default:
		return fmt.Errorf("unknown re-entry rule %q", t.Reentry)
	}
	if t.Yearly && t.Monthly {
		return errors.New("choose every year or every month, not both")
	}
	return nil
}

// Validate checks the settings are usable.
func (s Settings) Validate() error {
	q := s.QuietHours
	if q.Enabled {
		if q.Start < 0 || q.Start > 23 || q.End < 0 || q.End > 23 || q.Start == q.End {
			return errors.New("quiet hours: choose different start and end hours (0-23)")
		}
		if _, err := time.LoadLocation(q.Timezone); err != nil || q.Timezone == "" {
			return fmt.Errorf("quiet hours: unknown time zone %q", q.Timezone)
		}
	}
	if s.SmartSendingHours < 0 || s.SmartSendingHours > 24*30 {
		return errors.New("smart sending must be between 0 and 720 hours")
	}
	for i, c := range s.ExitConditions {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("exit condition %d: %w", i+1, err)
		}
	}
	for i, c := range s.FlowFilters {
		if c.Condition == CondOpenedLastEmail || c.Condition == CondClickedLastEmail {
			return fmt.Errorf("flow filter %d: \"last email\" can only be used in a step", i+1)
		}
		if err := c.Validate(); err != nil {
			return fmt.Errorf("flow filter %d: %w", i+1, err)
		}
	}
	return nil
}

// Validate checks the condition has what its kind needs.
func (c Cond) Validate() error {
	switch c.Condition {
	case CondHasTag, CondNotHasTag:
		if c.TagId <= 0 {
			return errors.New("choose the tag to check")
		}
	case CondHasPhone, CondSubscribed, CondOpenedLastEmail, CondClickedLastEmail:
	case CondAttrEquals, CondAttrSet:
		if strings.TrimSpace(c.Attribute) == "" {
			return errors.New("enter the attribute to check")
		}
	case CondOpenedCampaign, CondClickedCampaign:
		if c.TaskId <= 0 {
			return errors.New("choose the email campaign")
		}
	default:
		return fmt.Errorf("unknown condition %q", c.Condition)
	}
	return nil
}

// ValidateSteps checks every step, including steps inside branches.
func ValidateSteps(steps []Step) error {
	if len(steps) == 0 {
		return errors.New("add at least one step")
	}
	if n := CountSteps(steps); n > MaxSteps {
		return fmt.Errorf("a workflow can have at most %d steps", MaxSteps)
	}
	return validateList(steps, "", 0)
}

func validateList(steps []Step, prefix string, depth int) error {
	for i, s := range steps {
		label := fmt.Sprintf("%s%d", prefix, i+1)
		if err := s.Validate(); err != nil {
			return fmt.Errorf("step %s: %w", label, err)
		}
		if s.Type == StepSplit {
			if depth+1 > MaxBranchDepth {
				return fmt.Errorf("step %s: branches can be nested at most %d levels", label, MaxBranchDepth)
			}
			yes, no := "Yes", "No"
			if s.SplitMode == SplitRandom {
				yes, no = "A", "B"
			}
			if err := validateList(s.Yes, label+" "+yes+" > ", depth+1); err != nil {
				return err
			}
			if err := validateList(s.No, label+" "+no+" > ", depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// CountSteps counts steps including those inside branches.
func CountSteps(steps []Step) int {
	n := 0
	for _, s := range steps {
		n++
		if s.Type == StepSplit {
			n += CountSteps(s.Yes) + CountSteps(s.No)
		}
	}
	return n
}

// Validate checks the step has what its type needs (not its branches).
func (s Step) Validate() error {
	if err := s.validateType(); err != nil {
		return err
	}
	if s.IsMessage() {
		switch s.Status {
		case "", MessageLive, MessageManual, MessageDraft:
		default:
			return fmt.Errorf("unknown message status %q", s.Status)
		}
		for i, c := range s.Filters {
			if err := c.Validate(); err != nil {
				return fmt.Errorf("message filter %d: %w", i+1, err)
			}
		}
	}
	return nil
}

func (s Step) validateType() error {
	switch s.Type {
	case StepWait:
		// 0 days is fine with a time of day: "wait until 10:00"
		if s.WaitValue < 0 || (s.WaitValue == 0 && s.AtTime == "") {
			return errors.New("wait time must be more than 0")
		}
		if _, err := WaitDuration(s.WaitValue, s.WaitUnit); err != nil {
			return err
		}
		if s.AtTime != "" || len(s.Weekdays) > 0 {
			if s.WaitUnit != "days" && s.WaitUnit != "" {
				return errors.New("a time of day or weekdays need the wait in days")
			}
			if s.AtTime != "" {
				if _, _, ok := parseClock(s.AtTime); !ok {
					return errors.New("time of day must look like 10:00")
				}
			}
			for _, d := range s.Weekdays {
				if d < 0 || d > 6 {
					return errors.New("weekdays must be 0 (Sunday) to 6 (Saturday)")
				}
			}
			if s.Timezone != "" {
				if _, err := time.LoadLocation(s.Timezone); err != nil {
					return fmt.Errorf("unknown time zone %q", s.Timezone)
				}
			}
		}
	case StepEmail:
		if s.TemplateId <= 0 {
			return errors.New("choose an email template")
		}
		if strings.TrimSpace(s.Subject) == "" {
			return errors.New("enter an email subject")
		}
		if strings.TrimSpace(s.Addresser) == "" {
			return errors.New("choose the sender mailbox")
		}
	case StepSMS:
		if strings.TrimSpace(s.Message) == "" {
			return errors.New("enter the SMS text")
		}
		if len(s.Message) > 1600 {
			return errors.New("SMS text is too long (max 1600 characters)")
		}
	case StepWhatsApp:
		if strings.TrimSpace(s.WaTemplate) == "" || strings.TrimSpace(s.WaLanguage) == "" {
			return errors.New("choose an approved WhatsApp template")
		}
	case StepNotify:
		if len(SplitList(s.NotifyTo)) == 0 {
			return errors.New("enter who to alert (staff email or phone)")
		}
		if strings.TrimSpace(s.Message) == "" {
			return errors.New("write the alert message")
		}
		switch s.NotifyChannel {
		case NotifyEmail:
			if strings.TrimSpace(s.Addresser) == "" {
				return errors.New("choose the mailbox the alert is sent from")
			}
		case NotifySMS:
		default:
			return errors.New("choose how to alert staff (email or SMS)")
		}
	case StepWebhook:
		u := strings.TrimSpace(s.WebhookURL)
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return errors.New("webhook URL must start with http:// or https://")
		}
		if _, err := ParseHeaders(s.WebhookHeaders); err != nil {
			return err
		}
	case StepAddGroup:
		if s.GroupId <= 0 {
			return errors.New("choose the group to add the contact to")
		}
	case StepDelField:
		if strings.TrimSpace(s.Attribute) == "" {
			return errors.New("enter the field to remove")
		}
		if s.Attribute == "phone" || s.Attribute == "name" {
			return errors.New("name and phone cannot be removed by a workflow")
		}
	case StepAddTag, StepRemoveTag:
		if s.TagId <= 0 {
			return errors.New("choose a tag")
		}
	case StepSetField:
		if strings.TrimSpace(s.Attribute) == "" {
			return errors.New("enter the field to update")
		}
		if strings.ContainsAny(s.Attribute, " {}") {
			return errors.New("field names cannot contain spaces or braces")
		}
	case StepCondition:
		return s.Cond.Validate()
	case StepSplit:
		switch s.SplitMode {
		case SplitCondition:
			return s.Cond.Validate()
		case SplitRandom:
			if s.Percent < 1 || s.Percent > 99 {
				return errors.New("A/B split: share for A must be 1-99%")
			}
		default:
			return errors.New("choose Yes/No or A/B for the split")
		}
	default:
		return fmt.Errorf("unknown step type %q", s.Type)
	}
	return nil
}

// SplitList splits a comma/semicolon/newline separated list and drops blanks.
func SplitList(v string) []string {
	f := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	out := make([]string, 0, len(f))
	for _, s := range f {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// WaitDuration converts a wait step to a duration.
func WaitDuration(value int, unit string) (time.Duration, error) {
	switch unit {
	case "minutes":
		return time.Duration(value) * time.Minute, nil
	case "hours":
		return time.Duration(value) * time.Hour, nil
	case "days", "":
		return time.Duration(value) * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("unknown wait unit %q", unit)
}

var dateLayouts = []string{
	"2006-01-02",
	"2006/01/02",
	"02/01/2006",
	"02-01-2006",
	"02.01.2006",
	"2006-01-02 15:04:05",
	time.RFC3339,
}

// ParseDate reads a date attribute value. Day-first formats are used for slashes
// (UAE/Pakistan style: 25/12/2026).
func ParseDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// DateTriggerKey reports whether a date trigger fires today for the given attribute
// value and returns a key that is unique per occurrence (so a contact enters once per
// due date, again next year for yearly triggers, and again when the date changes).
func DateTriggerKey(value string, offsetDays int, yearly, monthly bool, today time.Time) (string, bool) {
	d, ok := ParseDate(value)
	if !ok {
		return "", false
	}
	due := d.AddDate(0, 0, offsetDays)
	if monthly {
		// Same day every month; days past the end of a short month fire on its last day.
		last := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location()).Day()
		if due.Day() == today.Day() || (due.Day() > last && today.Day() == last) {
			return "d:" + today.Format("2006-01-02"), true
		}
		return "", false
	}
	if yearly {
		if due.Month() == today.Month() && due.Day() == today.Day() {
			return "d:" + today.Format("2006-01-02"), true
		}
		// Feb 29 dates fire on Feb 28 in non-leap years.
		if due.Month() == time.February && due.Day() == 29 && today.Month() == time.February && today.Day() == 28 && !isLeap(today.Year()) {
			return "d:" + today.Format("2006-01-02"), true
		}
		return "", false
	}
	if due.Format("2006-01-02") == today.Format("2006-01-02") {
		return "d:" + due.Format("2006-01-02"), true
	}
	return "", false
}

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// NextSendTime returns now when sending is allowed, otherwise the next moment the
// quiet-hours window opens.
func NextSendTime(q QuietHours, now time.Time) time.Time {
	if !q.Enabled || q.Start == q.End {
		return now
	}
	loc, err := time.LoadLocation(q.Timezone)
	if err != nil {
		return now
	}
	local := now.In(loc)
	h := local.Hour()
	allowed := false
	if q.Start < q.End {
		allowed = h >= q.Start && h < q.End
	} else { // window spans midnight, e.g. 20 -> 2
		allowed = h >= q.Start || h < q.End
	}
	if allowed {
		return now
	}
	open := time.Date(local.Year(), local.Month(), local.Day(), q.Start, 0, 0, 0, loc)
	if !open.After(local) {
		open = open.AddDate(0, 0, 1)
	}
	return open
}
