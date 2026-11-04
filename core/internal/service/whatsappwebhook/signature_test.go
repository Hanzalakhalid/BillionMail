package whatsappwebhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account"}`)
	mac := hmac.New(sha256.New, []byte("app-secret"))
	_, _ = mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if err := VerifySignature(body, signature, "app-secret"); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(append(body, 'x'), signature, "app-secret"); err == nil {
		t.Fatal("tampered body accepted")
	}
}

func TestVerifyChallenge(t *testing.T) {
	if challenge, ok := VerifyChallenge("subscribe", "token", "12345", "token"); !ok || challenge != "12345" {
		t.Fatal("valid challenge rejected")
	}
	if _, ok := VerifyChallenge("subscribe", "wrong", "12345", "token"); ok {
		t.Fatal("invalid verify token accepted")
	}
}
