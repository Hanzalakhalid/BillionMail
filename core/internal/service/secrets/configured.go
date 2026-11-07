package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/google/uuid"
)

// EnvelopeStore is implemented by both the AWS KMS store and the local
// development store. Production is deliberately required to use KMS.
type EnvelopeStore interface {
	Encrypt(context.Context, uuid.UUID, []byte) (Envelope, error)
	Decrypt(context.Context, uuid.UUID, Envelope) ([]byte, error)
}

func NewConfiguredStore(ctx context.Context) (EnvelopeStore, error) {
	if keyARN := strings.TrimSpace(os.Getenv("SAAS_KMS_KEY_ARN")); keyARN != "" {
		cfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("load AWS configuration: %w", err)
		}
		return NewKMSStore(kms.NewFromConfig(cfg), keyARN)
	}
	if strings.EqualFold(os.Getenv("SAAS_ENV"), "production") {
		return nil, errors.New("SAAS_KMS_KEY_ARN is required in production")
	}
	secret := strings.TrimSpace(os.Getenv("SAAS_LOCAL_MASTER_KEY"))
	if secret == "" {
		// This fallback exists only to make a checked-out development instance
		// usable. Deployments must set SAAS_ENV=production and use KMS.
		secret = "billionmail-local-development-secret-change-me"
	}
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil || len(key) != 32 {
		sum := sha256.Sum256([]byte(secret))
		key = sum[:]
	}
	return &localStore{masterKey: key}, nil
}

// NewPersistentStore rejects the shared development fallback. BYOK credentials
// must remain decryptable only with an explicitly configured deployment key.
func NewPersistentStore(ctx context.Context) (EnvelopeStore, error) {
	if strings.TrimSpace(os.Getenv("SAAS_KMS_KEY_ARN")) == "" && strings.TrimSpace(os.Getenv("SAAS_LOCAL_MASTER_KEY")) == "" {
		return nil, errors.New("SAAS_LOCAL_MASTER_KEY or SAAS_KMS_KEY_ARN is required for provider credentials")
	}
	return NewConfiguredStore(ctx)
}

type localStore struct{ masterKey []byte }

func (s *localStore) Encrypt(_ context.Context, companyID uuid.UUID, plaintext []byte) (Envelope, error) {
	if companyID == uuid.Nil || len(plaintext) == 0 || len(plaintext) > maxSecretBytes {
		return Envelope{}, errors.New("company and a secret up to 64 KiB are required")
	}
	dataKey := make([]byte, 32)
	if _, err := rand.Read(dataKey); err != nil {
		return Envelope{}, err
	}
	defer zero(dataKey)
	ciphertext, err := seal(dataKey, plaintext, []byte(companyID.String()))
	if err != nil {
		return Envelope{}, err
	}
	encryptedKey, err := seal(s.masterKey, dataKey, []byte("billionmail:"+companyID.String()))
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Ciphertext: ciphertext, EncryptedDataKey: encryptedKey, KeyARN: "local:v1"}, nil
}

func (s *localStore) Decrypt(_ context.Context, companyID uuid.UUID, envelope Envelope) ([]byte, error) {
	if companyID == uuid.Nil || envelope.KeyARN != "local:v1" {
		return nil, errors.New("invalid local encrypted secret envelope")
	}
	dataKey, err := open(s.masterKey, envelope.EncryptedDataKey, []byte("billionmail:"+companyID.String()))
	if err != nil {
		return nil, err
	}
	defer zero(dataKey)
	return open(dataKey, envelope.Ciphertext, []byte(companyID.String()))
}

func seal(key, plaintext, additionalData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, gcm.Seal(nil, nonce, plaintext, additionalData)...), nil
}

func open(key, value, additionalData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(value) <= gcm.NonceSize() {
		return nil, errors.New("encrypted secret is truncated")
	}
	plaintext, err := gcm.Open(nil, value[:gcm.NonceSize()], value[gcm.NonceSize():], additionalData)
	if err != nil {
		return nil, errors.New("encrypted secret authentication failed")
	}
	return plaintext, nil
}
