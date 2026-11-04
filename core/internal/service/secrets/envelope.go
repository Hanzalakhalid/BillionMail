package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/google/uuid"
)

const maxSecretBytes = 64 << 10

type KMSClient interface {
	GenerateDataKey(context.Context, *kms.GenerateDataKeyInput, ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error)
	Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error)
}

type Envelope struct {
	Ciphertext       []byte
	EncryptedDataKey []byte
	KeyARN           string
}

type Store struct {
	client KMSClient
	keyARN string
}

func NewKMSStore(client KMSClient, keyARN string) (*Store, error) {
	if client == nil || keyARN == "" {
		return nil, errors.New("KMS client and key ARN are required")
	}
	return &Store{client: client, keyARN: keyARN}, nil
}

func (s *Store) Encrypt(ctx context.Context, companyID uuid.UUID, plaintext []byte) (Envelope, error) {
	if companyID == uuid.Nil || len(plaintext) == 0 || len(plaintext) > maxSecretBytes {
		return Envelope{}, errors.New("company and a secret up to 64 KiB are required")
	}
	key, err := s.client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId: aws.String(s.keyARN), KeySpec: types.DataKeySpecAes256,
		EncryptionContext: encryptionContext(companyID),
	})
	if err != nil {
		return Envelope{}, fmt.Errorf("generate KMS data key: %w", err)
	}
	defer zero(key.Plaintext)
	if len(key.Plaintext) != 32 || len(key.CiphertextBlob) == 0 {
		return Envelope{}, errors.New("KMS returned an invalid data key")
	}
	block, err := aes.NewCipher(key.Plaintext)
	if err != nil {
		return Envelope{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Envelope{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return Envelope{}, err
	}
	additionalData := []byte(companyID.String())
	sealed := gcm.Seal(nil, nonce, plaintext, additionalData)
	ciphertext := append(nonce, sealed...)
	return Envelope{Ciphertext: ciphertext, EncryptedDataKey: key.CiphertextBlob, KeyARN: s.keyARN}, nil
}

func (s *Store) Decrypt(ctx context.Context, companyID uuid.UUID, envelope Envelope) ([]byte, error) {
	if companyID == uuid.Nil || len(envelope.Ciphertext) == 0 || len(envelope.EncryptedDataKey) == 0 || envelope.KeyARN != s.keyARN {
		return nil, errors.New("invalid encrypted secret envelope")
	}
	key, err := s.client.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob: envelope.EncryptedDataKey, KeyId: aws.String(envelope.KeyARN),
		EncryptionContext: encryptionContext(companyID),
	})
	if err != nil {
		return nil, fmt.Errorf("decrypt KMS data key: %w", err)
	}
	defer zero(key.Plaintext)
	block, err := aes.NewCipher(key.Plaintext)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(envelope.Ciphertext) <= gcm.NonceSize() {
		return nil, errors.New("encrypted secret is truncated")
	}
	nonce, ciphertext := envelope.Ciphertext[:gcm.NonceSize()], envelope.Ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(companyID.String()))
	if err != nil {
		return nil, errors.New("encrypted secret authentication failed")
	}
	return plaintext, nil
}

func encryptionContext(companyID uuid.UUID) map[string]string {
	return map[string]string{"application": "billionmail", "company_id": companyID.String()}
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
