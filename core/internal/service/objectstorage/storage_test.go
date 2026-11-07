package objectstorage

import "testing"

func TestValidateKey(t *testing.T) {
	for _, key := range []string{"companies/123/assets/image.png", "companies/123/report.pdf"} {
		if err := ValidateKey(key); err != nil {
			t.Errorf("valid key %q rejected: %v", key, err)
		}
	}
	for _, key := range []string{"", "/companies/123/a", "companies/123/../a", "companies/123//a", "other/123/a", "companies\\123\\a", "companies/123"} {
		if err := ValidateKey(key); err == nil {
			t.Errorf("unsafe key %q accepted", key)
		}
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
