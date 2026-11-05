package workflow

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func TestTriggerValidate(t *testing.T) {
	cases := []struct {
		name    string
		trigger Trigger
		wantErr bool
	}{
		{"group join ok", Trigger{Type: TriggerGroupJoin, GroupId: 3}, false},
		{"group join missing group", Trigger{Type: TriggerGroupJoin}, true},
		{"tag ok", Trigger{Type: TriggerTagAdded, TagId: 7}, false},
		{"tag missing", Trigger{Type: TriggerTagAdded}, true},
		{"opened ok", Trigger{Type: TriggerEmailOpened, TaskId: 9}, false},
		{"clicked missing campaign", Trigger{Type: TriggerEmailClicked}, true},
		{"date ok", Trigger{Type: TriggerDate, Attribute: "last_visit", OffsetDays: 180}, false},
		{"date missing attribute", Trigger{Type: TriggerDate, Attribute: "  "}, true},
		{"date offset too big", Trigger{Type: TriggerDate, Attribute: "x", OffsetDays: 5000}, true},
		{"unknown", Trigger{Type: "nope"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.trigger.Validate(); (err != nil) != c.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestStepValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{"wait ok", Step{Type: StepWait, WaitValue: 1, WaitUnit: "days"}, false},
		{"wait zero", Step{Type: StepWait, WaitValue: 0, WaitUnit: "days"}, true},
		{"wait bad unit", Step{Type: StepWait, WaitValue: 1, WaitUnit: "weeks"}, true},
		{"email ok", Step{Type: StepEmail, TemplateId: 1, Subject: "Hi", Addresser: "a@b.com"}, false},
		{"email no sender", Step{Type: StepEmail, TemplateId: 1, Subject: "Hi"}, true},
		{"email no template", Step{Type: StepEmail, Subject: "Hi", Addresser: "a@b.com"}, true},
		{"sms ok", Step{Type: StepSMS, Message: "Hello {name}"}, false},
		{"sms empty", Step{Type: StepSMS, Message: " "}, true},
		{"whatsapp ok", Step{Type: StepWhatsApp, WaTemplate: "review", WaLanguage: "en"}, false},
		{"whatsapp no language", Step{Type: StepWhatsApp, WaTemplate: "review"}, true},
		{"add tag ok", Step{Type: StepAddTag, Cond: Cond{TagId: 2}}, false},
		{"remove tag missing", Step{Type: StepRemoveTag}, true},
		{"set field ok", Step{Type: StepSetField, Cond: Cond{Attribute: "last_visit", Value: "{today}"}}, false},
		{"set field bad name", Step{Type: StepSetField, Cond: Cond{Attribute: "last visit"}}, true},
		{"notify email ok", Step{Type: StepNotify, NotifyChannel: NotifyEmail, NotifyTo: "a@b.com", Message: "New lead {email}", Addresser: "x@y.com"}, false},
		{"notify email no sender", Step{Type: StepNotify, NotifyChannel: NotifyEmail, NotifyTo: "a@b.com", Message: "hi"}, true},
		{"notify no recipients", Step{Type: StepNotify, NotifyChannel: NotifySMS, NotifyTo: " , ", Message: "hi"}, true},
		{"notify sms ok", Step{Type: StepNotify, NotifyChannel: NotifySMS, NotifyTo: "0501234567", Message: "hi"}, false},
		{"split condition ok", Step{Type: StepSplit, SplitMode: SplitCondition, Cond: Cond{Condition: CondOpenedLastEmail}}, false},
		{"split condition missing", Step{Type: StepSplit, SplitMode: SplitCondition}, true},
		{"split random ok", Step{Type: StepSplit, SplitMode: SplitRandom, Percent: 50}, false},
		{"split random 100", Step{Type: StepSplit, SplitMode: SplitRandom, Percent: 100}, true},
		{"split no mode", Step{Type: StepSplit}, true},
		{"condition campaign missing", Step{Type: StepCondition, Cond: Cond{Condition: CondOpenedCampaign}}, true},
		{"condition has tag", Step{Type: StepCondition, Cond: Cond{Condition: CondHasTag, TagId: 1}}, false},
		{"condition has tag missing", Step{Type: StepCondition, Cond: Cond{Condition: CondHasTag}}, true},
		{"condition phone", Step{Type: StepCondition, Cond: Cond{Condition: CondHasPhone}}, false},
		{"condition attr missing", Step{Type: StepCondition, Cond: Cond{Condition: CondAttrEquals}}, true},
		{"condition unknown", Step{Type: StepCondition, Cond: Cond{Condition: "x"}}, true},
		{"unknown step", Step{Type: "fly"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.step.Validate(); (err != nil) != c.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestValidateSteps(t *testing.T) {
	if err := ValidateSteps(nil); err == nil {
		t.Fatal("empty steps should fail")
	}
	many := make([]Step, MaxSteps+1)
	for i := range many {
		many[i] = Step{Type: StepAddTag, Cond: Cond{TagId: 1}}
	}
	if err := ValidateSteps(many); err == nil {
		t.Fatal("too many steps should fail")
	}
	err := ValidateSteps([]Step{{Type: StepAddTag, Cond: Cond{TagId: 1}}, {Type: StepSMS}})
	if err == nil || err.Error()[:6] != "step 2" {
		t.Fatalf("want error naming step 2, got %v", err)
	}
}

func TestWaitDuration(t *testing.T) {
	cases := []struct {
		value int
		unit  string
		want  time.Duration
	}{
		{30, "minutes", 30 * time.Minute},
		{2, "hours", 2 * time.Hour},
		{3, "days", 72 * time.Hour},
		{1, "", 24 * time.Hour},
	}
	for _, c := range cases {
		got, err := WaitDuration(c.value, c.unit)
		if err != nil || got != c.want {
			t.Fatalf("WaitDuration(%d, %q) = %v, %v; want %v", c.value, c.unit, got, err, c.want)
		}
	}
	if _, err := WaitDuration(1, "weeks"); err == nil {
		t.Fatal("unknown unit should fail")
	}
}

func TestParseDate(t *testing.T) {
	for _, v := range []string{"2026-05-04", "2026/05/04", "04/05/2026", "04-05-2026", "04.05.2026", "2026-05-04 10:30:00"} {
		got, ok := ParseDate(v)
		if !ok || got.Format("2006-01-02") != "2026-05-04" {
			t.Fatalf("ParseDate(%q) = %v, %v", v, got, ok)
		}
	}
	for _, v := range []string{"", "tomorrow", "2026-13-40"} {
		if _, ok := ParseDate(v); ok {
			t.Fatalf("ParseDate(%q) should fail", v)
		}
	}
}

func TestDateTriggerKey(t *testing.T) {
	// 180 days after the last visit
	key, ok := DateTriggerKey("2026-05-08", 180, false, false, day("2026-11-04"))
	if !ok || key != "d:2026-11-04" {
		t.Fatalf("recall: got %q %v", key, ok)
	}
	if _, ok := DateTriggerKey("2026-05-08", 180, false, false, day("2026-11-05")); ok {
		t.Fatal("recall should not fire the next day")
	}
	// 3 days before an appointment
	if _, ok := DateTriggerKey("07/11/2026", -3, false, false, day("2026-11-04")); !ok {
		t.Fatal("reminder 3 days before should fire")
	}
	// Birthday, yearly
	key, ok = DateTriggerKey("1990-11-04", 0, true, false, day("2026-11-04"))
	if !ok || key != "d:2026-11-04" {
		t.Fatalf("birthday: got %q %v", key, ok)
	}
	if _, ok := DateTriggerKey("1990-11-05", 0, true, false, day("2026-11-04")); ok {
		t.Fatal("birthday on another day should not fire")
	}
	// Feb 29 birthday fires on Feb 28 in non-leap years
	if _, ok := DateTriggerKey("2000-02-29", 0, true, false, day("2027-02-28")); !ok {
		t.Fatal("leap birthday should fire on Feb 28")
	}
	if _, ok := DateTriggerKey("not a date", 0, true, false, day("2026-11-04")); ok {
		t.Fatal("bad date should not fire")
	}
}

func TestWaitText(t *testing.T) {
	cases := map[string]string{
		waitText(1, "minutes"): "1 minute",
		waitText(3, "hours"):   "3 hours",
		waitText(1, ""):        "1 day",
		waitText(7, "days"):    "7 days",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("waitText = %q, want %q", got, want)
		}
	}
}

func TestAttrEquals(t *testing.T) {
	attribs := map[string]string{"treatment": " Whitening "}
	if ok, _ := attrEquals(attribs, "treatment", "whitening"); !ok {
		t.Fatal("should match ignoring case and spaces")
	}
	if ok, why := attrEquals(attribs, "treatment", "braces"); ok || why == "" {
		t.Fatal("should not match")
	}
	if ok, _ := attrEquals(attribs, "missing", ""); !ok {
		t.Fatal("missing attribute equals empty value")
	}
}
