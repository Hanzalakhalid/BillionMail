package queue

import "testing"

func TestWorkloadQueue(t *testing.T) {
	tests := map[string]string{
		"identity.email_verification.requested": "notifications",
		"company.invitation.created":            "notifications",
		"whatsapp.webhook.received":             "webhooks",
		"whatsapp.message.send":                 "delivery",
		"analytics.refresh":                     "default",
	}
	for eventType, expected := range tests {
		if actual := workloadQueue(eventType); actual != expected {
			t.Errorf("workloadQueue(%q)=%q want %q", eventType, actual, expected)
		}
	}
}
