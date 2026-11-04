package tenancy

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestTenantContextRoundTrip(t *testing.T) {
	want := TenantContext{UserID: uuid.New(), CompanyID: uuid.New(), MembershipID: uuid.New(), Role: "manager"}
	got, err := Require(WithContext(context.Background(), want))
	if err != nil {
		t.Fatalf("Require() error = %v", err)
	}
	if got != want {
		t.Fatalf("Require() = %#v, want %#v", got, want)
	}
}

func TestRequireRejectsMissingTenant(t *testing.T) {
	if _, err := Require(context.Background()); err != ErrTenantContextMissing {
		t.Fatalf("Require() error = %v, want %v", err, ErrTenantContextMissing)
	}
}

func TestRolePermissions(t *testing.T) {
	tests := []struct {
		role, permission string
		allowed          bool
	}{
		{"owner", "billing.manage", true},
		{"admin", "flow.publish", true},
		{"manager", "conversation.assign", true},
		{"agent", "conversation.reply", true},
		{"agent", "flow.publish", false},
		{"viewer", "analytics.read", true},
		{"viewer", "contact.update", false},
	}
	for _, test := range tests {
		err := Authorize(TenantContext{Role: test.role}, test.permission)
		if (err == nil) != test.allowed {
			t.Errorf("Authorize(%s,%s) error=%v", test.role, test.permission, err)
		}
	}
}

func TestOpaqueTokensAreUniqueAndHashed(t *testing.T) {
	one, oneHash, err := newOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	two, twoHash, err := newOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if one == two {
		t.Fatal("opaque tokens must be unique")
	}
	if string(oneHash) == string(twoHash) {
		t.Fatal("token hashes must be unique")
	}
	if one == string(oneHash) {
		t.Fatal("stored hash must not equal raw token")
	}
}

func TestInvitationTokenIsDeterministicAndDoesNotExposeSecret(t *testing.T) {
	id := uuid.New()
	secret := "01234567890123456789012345678901"
	one, oneHash, err := BuildInvitationToken(id, secret)
	if err != nil {
		t.Fatal(err)
	}
	two, twoHash, err := BuildInvitationToken(id, secret)
	if err != nil {
		t.Fatal(err)
	}
	if one != two || string(oneHash) != string(twoHash) {
		t.Fatal("invitation token must be deterministic")
	}
	if strings.Contains(one, secret) {
		t.Fatal("invitation token exposed signing secret")
	}
	other, _, _ := BuildInvitationToken(uuid.New(), secret)
	if one == other {
		t.Fatal("different invitations must have different tokens")
	}
}
