package identity

import (
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	for _, valid := range []string{"LongEnough123", "AnotherSecure9Password"} {
		if err := validatePassword(valid); err != nil {
			t.Errorf("valid password rejected: %v", err)
		}
	}
	for _, invalid := range []string{"short1A", "alllowercase123", "ALLUPPERCASE123", "NoNumbersHere"} {
		if err := validatePassword(invalid); err == nil {
			t.Errorf("invalid password accepted: %q", invalid)
		}
	}
}

func TestActionTokens(t *testing.T) {
	id := uuid.New()
	secret := strings.Repeat("s", 32)
	one, oneHash, err := BuildActionToken(id, "verify-email", secret)
	if err != nil {
		t.Fatal(err)
	}
	two, twoHash, _ := BuildActionToken(id, "verify-email", secret)
	if one != two || string(oneHash) != string(twoHash) {
		t.Fatal("action token is not deterministic")
	}
	other, _, _ := BuildActionToken(id, "reset-password", secret)
	if one == other {
		t.Fatal("token purpose is not domain separated")
	}
}

func TestSlugify(t *testing.T) {
	if got := slugify("  ABC Medical & Health  "); got != "abc-medical-health" {
		t.Fatalf("slugify=%q", got)
	}
}
