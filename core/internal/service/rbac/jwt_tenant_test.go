package rbac

import "testing"

func TestTenantAwareCompatibilityPaths(t *testing.T) {
	for _, path := range []string{"/api/v1/companies/list", "/api/v1/flows/list", "/api/logout"} {
		if !isTenantAwarePath(path) {
			t.Errorf("expected tenant-aware path: %s", path)
		}
	}
	for _, path := range []string{"/api/contacts/list", "/api/overview/stat", "/api/whatsapp/send"} {
		if isTenantAwarePath(path) {
			t.Errorf("legacy path bypassed compatibility gate: %s", path)
		}
	}
}
