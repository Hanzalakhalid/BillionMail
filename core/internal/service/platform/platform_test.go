package platform

import "testing"

func TestEntitlements(t *testing.T) {
	entitlements := Entitlements{"messages": 100, "ai_tokens": 0, "active_flows": -1}
	tests := []struct {
		metric             string
		current, requested int64
		want               bool
	}{
		{"messages", 90, 10, true}, {"messages", 90, 11, false},
		{"ai_tokens", 0, 1, false}, {"active_flows", 999, 1, true},
		{"unknown", 0, 1, false},
	}
	for _, test := range tests {
		if got := entitlements.Allows(test.metric, test.current, test.requested); got != test.want {
			t.Errorf("Allows(%s,%d,%d)=%v want %v", test.metric, test.current, test.requested, got, test.want)
		}
	}
}
