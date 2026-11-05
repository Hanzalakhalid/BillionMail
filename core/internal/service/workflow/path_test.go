package workflow

import (
	"strings"
	"testing"
	"time"

	"billionmail-core/internal/model/entity"
)

// flow:
// 0 wait
// 1 split  yes: [0 sms, 1 split yes:[0 add_tag] no:[]]   no: [0 email]
// 2 sms
func sampleFlow() []Step {
	return []Step{
		{Type: StepWait, WaitValue: 1, WaitUnit: "days"},
		{Type: StepSplit, SplitMode: SplitCondition, Cond: Cond{Condition: CondHasPhone},
			Yes: []Step{
				{Type: StepSMS, Message: "hi"},
				{Type: StepSplit, SplitMode: SplitRandom, Percent: 50,
					Yes: []Step{{Type: StepAddTag, Cond: Cond{TagId: 1}}},
				},
			},
			No: []Step{{Type: StepEmail, TemplateId: 1, Subject: "s", Addresser: "a@b.c"}},
		},
		{Type: StepSMS, Message: "bye"},
	}
}

func TestStepAt(t *testing.T) {
	f := sampleFlow()
	cases := map[string]string{
		"0":         StepWait,
		"1":         StepSplit,
		"1/y/0":     StepSMS,
		"1/y/1/y/0": StepAddTag,
		"1/n/0":     StepEmail,
		"2":         StepSMS,
	}
	for path, want := range cases {
		s, ok := StepAt(f, path)
		if !ok || s.Type != want {
			t.Fatalf("StepAt(%q) = %v %v, want %s", path, s.Type, ok, want)
		}
	}
	for _, bad := range []string{"", "3", "1/y", "0/y/0", "1/x/0", "1/n/1", "a"} {
		if _, ok := StepAt(f, bad); ok {
			t.Fatalf("StepAt(%q) should fail", bad)
		}
	}
}

func TestNextPath(t *testing.T) {
	f := sampleFlow()
	cases := map[string]string{
		"0":         "1",
		"1/y/0":     "1/y/1",
		"1/y/1/y/0": "2", // end of inner branch, end of outer branch -> after outer split
		"1/n/0":     "2",
		"2":         "",
	}
	for path, want := range cases {
		if got := NextPath(f, path); got != want {
			t.Fatalf("NextPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestEnterBranch(t *testing.T) {
	f := sampleFlow()
	if got := EnterBranch(f, "1", BranchYes); got != "1/y/0" {
		t.Fatalf("yes branch: %q", got)
	}
	if got := EnterBranch(f, "1/y/1", BranchNo); got != "2" {
		t.Fatalf("empty no branch should skip to after the outer split, got %q", got)
	}
	if TopIndex("1/y/1/y/0") != 1 || TopIndex("") != 0 {
		t.Fatal("TopIndex")
	}
}

func TestValidateNestedSteps(t *testing.T) {
	if err := ValidateSteps(sampleFlow()); err != nil {
		t.Fatalf("sample flow should be valid: %v", err)
	}
	f := sampleFlow()
	f[1].No[0].Subject = ""
	err := ValidateSteps(f)
	if err == nil || !strings.HasPrefix(err.Error(), "step 2 No > 1:") {
		t.Fatalf("want error naming the branch step, got %v", err)
	}
	if CountSteps(sampleFlow()) != 7 {
		t.Fatalf("CountSteps = %d", CountSteps(sampleFlow()))
	}
	// 4 levels of nesting is too deep
	deep := []Step{{Type: StepSMS, Message: "x"}}
	for i := 0; i < 4; i++ {
		deep = []Step{{Type: StepSplit, SplitMode: SplitRandom, Percent: 50, Yes: deep}}
	}
	if err := ValidateSteps(deep); err == nil {
		t.Fatal("too deep nesting should fail")
	}
}

func TestSettingsValidate(t *testing.T) {
	ok := Settings{QuietHours: QuietHours{Enabled: true, Start: 9, End: 21, Timezone: "Asia/Dubai"}, SmartSendingHours: 16,
		ExitConditions: []Cond{{Condition: CondHasTag, TagId: 3}}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid settings: %v", err)
	}
	bad := []Settings{
		{QuietHours: QuietHours{Enabled: true, Start: 9, End: 9, Timezone: "Asia/Dubai"}},
		{QuietHours: QuietHours{Enabled: true, Start: 9, End: 21, Timezone: "Mars/Base"}},
		{SmartSendingHours: -1},
		{ExitConditions: []Cond{{Condition: CondHasTag}}},
	}
	for i, s := range bad {
		if err := s.Validate(); err == nil {
			t.Fatalf("bad settings %d should fail", i)
		}
	}
	if err := (Settings{}).Validate(); err != nil {
		t.Fatalf("empty settings are fine: %v", err)
	}
}

func TestNextSendTime(t *testing.T) {
	dubai, _ := time.LoadLocation("Asia/Dubai")
	q := QuietHours{Enabled: true, Start: 9, End: 21, Timezone: "Asia/Dubai"}
	at := func(h, m int) time.Time { return time.Date(2026, 11, 4, h, m, 0, 0, dubai) }

	if got := NextSendTime(q, at(10, 0)); !got.Equal(at(10, 0)) {
		t.Fatalf("inside window should send now, got %v", got)
	}
	if got := NextSendTime(q, at(23, 30)); !got.Equal(time.Date(2026, 11, 5, 9, 0, 0, 0, dubai)) {
		t.Fatalf("late night should wait for 9am tomorrow, got %v", got)
	}
	if got := NextSendTime(q, at(6, 0)); !got.Equal(at(9, 0)) {
		t.Fatalf("early morning should wait for 9am today, got %v", got)
	}
	// Window over midnight: 20:00 -> 02:00
	night := QuietHours{Enabled: true, Start: 20, End: 2, Timezone: "Asia/Dubai"}
	if got := NextSendTime(night, at(1, 0)); !got.Equal(at(1, 0)) {
		t.Fatalf("01:00 is inside a 20-02 window, got %v", got)
	}
	if got := NextSendTime(night, at(12, 0)); !got.Equal(at(20, 0)) {
		t.Fatalf("noon should wait for 20:00, got %v", got)
	}
	if got := NextSendTime(QuietHours{}, at(3, 0)); !got.Equal(at(3, 0)) {
		t.Fatal("disabled quiet hours never wait")
	}
}

func TestExpandValue(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.Local)
	c := entity.Contact{Email: "a@b.com", Attribs: map[string]string{"name": "Sara"}}
	cases := map[string]string{
		"{today}":          "2026-11-04",
		"{today+7}":        "2026-11-11",
		"{today-4}":        "2026-10-31",
		"booked by {name}": "booked by Sara",
		"plain":            "plain",
	}
	for in, want := range cases {
		if got := ExpandValue(in, c, now); got != want {
			t.Fatalf("ExpandValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitList(t *testing.T) {
	got := SplitList(" a@b.com, c@d.com ;\n0501234567,, ")
	if len(got) != 3 || got[0] != "a@b.com" || got[2] != "0501234567" {
		t.Fatalf("SplitList = %v", got)
	}
}
