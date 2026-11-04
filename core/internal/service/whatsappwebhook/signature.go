package whatsappwebhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidSignature = errors.New("invalid Meta webhook signature")

func VerifySignature(body []byte, signatureHeader, appSecret string) error {
	if len(body) == 0 || appSecret == "" || !strings.HasPrefix(signatureHeader, "sha256=") {
		return ErrInvalidSignature
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signatureHeader, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return ErrInvalidSignature
	}
	return nil
}

func VerifyChallenge(mode, verifyToken, challenge, configuredToken string) (string, bool) {
	if mode != "subscribe" || challenge == "" || verifyToken == "" || configuredToken == "" {
		return "", false
	}
	if !hmac.Equal([]byte(verifyToken), []byte(configuredToken)) {
		return "", false
	}
	return challenge, true
}
