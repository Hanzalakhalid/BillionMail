package workflow

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultTimezone is used for time-of-day waits when the step has none.
const DefaultTimezone = "Asia/Dubai"

// parseClock reads "HH:MM" (24h).
func parseClock(v string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(v), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// WaitUntil is when a wait step ends, Klaviyo-style:
//   - minutes/hours/days without extras: now + duration
//   - days with a time of day: N calendar days later at that time (N=0: the next time that clock time comes)
//   - weekdays: pushed forward until it lands on an allowed weekday
func WaitUntil(s Step, now time.Time) time.Time {
	d, err := WaitDuration(s.WaitValue, s.WaitUnit)
	if err != nil {
		d = 24 * time.Hour
	}
	if s.AtTime == "" && len(s.Weekdays) == 0 {
		return now.Add(d)
	}

	tz := s.Timezone
	if tz == "" {
		tz = DefaultTimezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}
	local := now.In(loc)

	var due time.Time
	if h, m, ok := parseClock(s.AtTime); ok {
		due = time.Date(local.Year(), local.Month(), local.Day(), h, m, 0, 0, loc).AddDate(0, 0, s.WaitValue)
		if !due.After(local) {
			due = due.AddDate(0, 0, 1)
		}
	} else {
		due = local.Add(d)
	}

	if len(s.Weekdays) > 0 {
		allowed := map[time.Weekday]bool{}
		for _, w := range s.Weekdays {
			allowed[time.Weekday(w)] = true
		}
		for i := 0; i < 7 && !allowed[due.Weekday()]; i++ {
			due = due.AddDate(0, 0, 1)
		}
	}
	return due
}

// ParseHeaders reads webhook headers written as "Name: value", one per line.
func ParseHeaders(v string) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(v, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" || strings.ContainsAny(name, " \t") {
			return nil, fmt.Errorf("webhook header %q must look like Name: value", line)
		}
		out[name] = strings.TrimSpace(value)
	}
	return out, nil
}
