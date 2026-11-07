package objectstorage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestValidateKey(t *testing.T) {
	for _, key := range []string{"companies/719e33db-7455-4099-a799-9fb835fdf579/assets/image.png", "companies/719e33db-7455-4099-a799-9fb835fdf579/report.pdf"} {
		if err := ValidateKey(key); err != nil {
			t.Errorf("valid key %q rejected: %v", key, err)
		}
	}
	for _, key := range []string{"", "/companies/123/a", "companies/123/../a", "companies/123//a", "other/123/a", "companies\\123\\a", "companies/123", "companies/123/a", "companies/00000000-0000-0000-0000-000000000000/a"} {
		if err := ValidateKey(key); err == nil {
			t.Errorf("unsafe key %q accepted", key)
		}
	}
}

func TestSignedURLsUsePublicOrigin(t *testing.T) {
	store, err := New(context.Background(), Config{Bucket: "marketing-assets", Region: "us-east-1", Endpoint: "http://minio:9000", PublicEndpoint: "https://media.example.test", AccessKey: "test-user", SecretKey: "test-key", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	key := "companies/719e33db-7455-4099-a799-9fb835fdf579/assets/product.png"
	for _, operation := range []func() (string, error){
		func() (string, error) { return store.PresignPut(context.Background(), key, "image/png", time.Minute) },
		func() (string, error) { return store.PresignGet(context.Background(), key, time.Minute) },
	} {
		signed, err := operation()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(signed)
		if err != nil || parsed.Host != "media.example.test" || parsed.Path != "/marketing-assets/"+key || !strings.Contains(parsed.RawQuery, "X-Amz-Signature=") {
			t.Fatalf("unexpected signed URL: %s err=%v", signed, err)
		}
	}
	for _, ttl := range []time.Duration{0, time.Millisecond, 2 * time.Hour} {
		if _, err := store.PresignGet(context.Background(), key, ttl); err == nil {
			t.Fatalf("accepted ttl=%s", ttl)
		}
	}
}

func TestEndpointRejectsEmbeddedCredentials(t *testing.T) {
	for _, endpoint := range []string{"http://user:password@minio:9000", "https://", "https://media.test/path", "https://media.test?secret=test", "ftp://minio:9000"} {
		config := Config{Bucket: "assets", Region: "us-east-1", PublicEndpoint: endpoint}
		if config.Validate() == nil {
			t.Fatalf("accepted endpoint %s", endpoint)
		}
	}
}

func TestUploadPolicyEnforcesTypeAndSize(t *testing.T) {
	store, err := New(context.Background(), Config{Bucket: "assets", Region: "us-east-1", PublicEndpoint: "https://media.example.test", AccessKey: "test", SecretKey: "test", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	key := "companies/719e33db-7455-4099-a799-9fb835fdf579/quarantine/image"
	form, err := store.PresignUpload(context.Background(), key, "image/png", 4096, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	encoded := form.Fields["policy"]
	if encoded == "" {
		encoded = form.Fields["Policy"]
	}
	policy, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Conditions []json.RawMessage `json:"conditions"`
	}
	if err = json.Unmarshal(policy, &document); err != nil {
		t.Fatal(err)
	}
	hasSize, hasMime, hasKey := false, false, false
	for _, condition := range document.Conditions {
		var size []any
		if json.Unmarshal(condition, &size) == nil && len(size) == 3 && size[0] == "content-length-range" && size[1] == float64(1) && size[2] == float64(4096) {
			hasSize = true
		}
		var field map[string]string
		if json.Unmarshal(condition, &field) == nil {
			if field["Content-Type"] == "image/png" {
				hasMime = true
			}
			if field["key"] == key {
				hasKey = true
			}
		}
	}
	if !hasSize || !hasMime || !hasKey || form.Fields["Content-Type"] != "image/png" {
		t.Fatalf("upload policy lacks restrictions: %s", policy)
	}
}

func TestConfigValidate(t *testing.T) {
	valid := Config{Bucket: "marketing-assets", Region: "us-east-1", Endpoint: "http://minio:9000", AccessKey: "user", SecretKey: "secret", PathStyle: true}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	valid.SecretKey = ""
	if valid.Validate() == nil {
		t.Fatal("half-configured credentials accepted")
	}
}
