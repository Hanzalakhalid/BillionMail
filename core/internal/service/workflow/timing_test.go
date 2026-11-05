package workflow

import (
	"testing"
	"time"
)

func TestWaitUntilPlain(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	got := WaitUntil(Step{Type: StepWait, WaitValue: 2, WaitUnit: "hours"}, now)
	if !got.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("plain wait = %v", got)
	}
}

func TestWaitUntilTimeOfDay(t *testing.T) {
	dubai, _ := time.LoadLocation("Asia/Dubai")
	at := func(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, dubai) }
	s := Step{Type: StepWait, WaitValue: 1, WaitUnit: "days", AtTime: "10:00", Timezone: "Asia/Dubai"}

	// Thu 5 Nov 23:30 + 1 day at 10:00 -> Fri 6 Nov 10:00 (calendar days, not 24h)
	if got := WaitUntil(s, at(5, 23, 30)); !got.Equal(at(6, 10, 0)) {
		t.Fatalf("1 day at 10:00 = %v", got)
	}
	// 0 days at 10:00 when it's 08:00 -> today 10:00; when it's 11:00 -> tomorrow 10:00
	s.WaitValue = 0
	if got := WaitUntil(s, at(5, 8, 0)); !got.Equal(at(5, 10, 0)) {
		t.Fatalf("0 days before 10:00 = %v", got)
	}
	if got := WaitUntil(s, at(5, 11, 0)); !got.Equal(at(6, 10, 0)) {
		t.Fatalf("0 days after 10:00 = %v", got)
	}
}

func TestWaitUntilWeekdays(t *testing.T) {
	dubai, _ := time.LoadLocation("Asia/Dubai")
	// Klaviyo's example: enter Tuesday, wait 2 days, only Mon/Wed/Fri -> Friday.
	tue := time.Date(2026, 11, 3, 9, 0, 0, 0, dubai)
	s := Step{Type: StepWait, WaitValue: 2, WaitUnit: "days", Weekdays: []int{1, 3, 5}, Timezone: "Asia/Dubai"}
	got := WaitUntil(s, tue)
	if got.Weekday() != time.Friday || got.Day() != 6 {
		t.Fatalf("weekday wait = %v (%s)", got, got.Weekday())
	}
	// Mon-Fri only, at 09:00: Friday 18:00 + 1 day -> Saturday -> pushed to Monday 09:00
	fri := time.Date(2026, 11, 6, 18, 0, 0, 0, dubai)
	s = Step{Type: StepWait, WaitValue: 1, WaitUnit: "days", AtTime: "09:00", Weekdays: []int{1, 2, 3, 4, 5}, Timezone: "Asia/Dubai"}
	if got := WaitUntil(s, fri); !got.Equal(time.Date(2026, 11, 9, 9, 0, 0, 0, dubai)) {
		t.Fatalf("weekday + time = %v", got)
	}
}

func TestStepValidateKlaviyoOptions(t *testing.T) {
	cases := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{"wait at time ok", Step{Type: StepWait, WaitValue: 1, WaitUnit: "days", AtTime: "10:30"}, false},
		{"wait at time bad", Step{Type: StepWait, WaitValue: 1, WaitUnit: "days", AtTime: "25:00"}, true},
		{"wait at time needs days", Step{Type: StepWait, WaitValue: 1, WaitUnit: "hours", AtTime: "10:00"}, true},
		{"wait bad weekday", Step{Type: StepWait, WaitValue: 1, WaitUnit: "days", Weekdays: []int{7}}, true},
		{"message manual ok", Step{Type: StepSMS, Message: "hi", Status: MessageManual}, false},
		{"message bad status", Step{Type: StepSMS, Message: "hi", Status: "paused"}, true},
		{"message filter bad", Step{Type: StepSMS, Message: "hi", Filters: []Cond{{Condition: CondHasTag}}}, true},
		{"webhook ok", Step{Type: StepWebhook, WebhookURL: "https://n8n.example.com/hook", WebhookHeaders: "X-Key: abc"}, false},
		{"webhook bad url", Step{Type: StepWebhook, WebhookURL: "n8n.example.com"}, true},
		{"webhook bad header", Step{Type: StepWebhook, WebhookURL: "https://x.io", WebhookHeaders: "no colon"}, true},
		{"add to group ok", Step{Type: StepAddGroup, GroupId: 2}, false},
		{"add to group missing", Step{Type: StepAddGroup}, true},
		{"remove field ok", Step{Type: StepDelField, Cond: Cond{Attribute: "coupon"}}, false},
		{"remove phone blocked", Step{Type: StepDelField, Cond: Cond{Attribute: "phone"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.step.Validate(); (err != nil) != c.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestTriggerReentryAndSettingsFilters(t *testing.T) {
	base := Trigger{Type: TriggerGroupJoin, GroupId: 1}
	for _, r := range []string{"", ReentryAlways, ReentryOnce} {
		tr := base
		tr.Reentry = r
		if err := tr.Validate(); err != nil {
			t.Fatalf("reentry %q: %v", r, err)
		}
	}
	tr := base
	tr.Reentry = ReentryAfter
	if err := tr.Validate(); err == nil {
		t.Fatal("re-enter after needs days")
	}
	tr.ReentryDays = 30
	if err := tr.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Trigger{Type: TriggerDate, Attribute: "x", Yearly: true, Monthly: true}).Validate(); err == nil {
		t.Fatal("yearly and monthly together should fail")
	}
	if err := (Settings{FlowFilters: []Cond{{Condition: CondOpenedLastEmail}}}).Validate(); err == nil {
		t.Fatal("last-email flow filter should fail")
	}
	if err := (Settings{FlowFilters: []Cond{{Condition: CondHasPhone}}}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMonthlyDateTrigger(t *testing.T) {
	if _, ok := DateTriggerKey("2026-01-15", 0, false, true, day("2026-11-15")); !ok {
		t.Fatal("monthly should fire on the same day of any month")
	}
	if _, ok := DateTriggerKey("2026-01-15", 0, false, true, day("2026-11-16")); ok {
		t.Fatal("monthly should not fire on another day")
	}
	// The 31st fires on the last day of a 30-day month
	if _, ok := DateTriggerKey("2026-01-31", 0, false, true, day("2026-11-30")); !ok {
		t.Fatal("31st should fire on Nov 30")
	}
}

func TestParseHeaders(t *testing.T) {
	h, err := ParseHeaders("Authorization: Bearer x\n\nX-Source:  billionmail ")
	if err != nil || h["Authorization"] != "Bearer x" || h["X-Source"] != "billionmail" {
		t.Fatalf("ParseHeaders = %v, %v", h, err)
	}
}
