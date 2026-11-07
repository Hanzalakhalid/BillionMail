package integrations

import (
	"encoding/json"
	"testing"
)

func TestValidateRejectsCredentialsInPublicMetadata(t *testing.T) {
	base := Input{Provider: "openai", Capability: "image", Name: "studio", Secret: "test-secret", Configuration: json.RawMessage(`{"model":"image-model"}`)}
	if err := Validate(base); err != nil {
		t.Fatalf("valid metadata rejected: %v", err)
	}
	for _, field := range []string{"api_key", "access_token", "password", "clientSecret"} {
		base.Configuration = json.RawMessage(`{"` + field + `":"unsafe"}`)
		if err := Validate(base); err == nil {
			t.Errorf("credential-shaped field %q accepted", field)
		}
	}
	base.Configuration = json.RawMessage(`{"headers":[{"AuthorizationToken":"unsafe"}]}`)
	if err := Validate(base); err == nil {
		t.Fatal("nested credential-shaped field accepted")
	}
}

func TestValidateRequiresStructuredConfiguration(t *testing.T) {
	input := Input{Provider: "runway", Capability: "video", Name: "primary", Secret: "test-secret"}
	input.Configuration = json.RawMessage(`[]`)
	if Validate(input) == nil {
		t.Fatal("array configuration accepted")
	}
	input.Configuration = json.RawMessage(`{"model":"gen4.5"}`)
	if err := Validate(input); err != nil {
		t.Fatal(err)
	}
}
