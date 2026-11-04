package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/google/uuid"
)

type fakeKMS struct {
	company string
}

func (f *fakeKMS) GenerateDataKey(_ context.Context, input *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	f.company = input.EncryptionContext["company_id"]
	return &kms.GenerateDataKeyOutput{Plaintext: []byte("0123456789abcdef0123456789abcdef"), CiphertextBlob: []byte("encrypted-data-key")}, nil
}

func (f *fakeKMS) Decrypt(_ context.Context, input *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	if input.EncryptionContext["company_id"] != f.company {
		return nil, errors.New("encryption context mismatch")
	}
	return &kms.DecryptOutput{Plaintext: []byte("0123456789abcdef0123456789abcdef")}, nil
}

func TestEnvelopeEncryptionIsTenantBound(t *testing.T) {
	client := &fakeKMS{}
	store, err := NewKMSStore(client, "arn:aws:kms:test:key/test")
	if err != nil {
		t.Fatal(err)
	}
	companyID := uuid.New()
	envelope, err := store.Encrypt(context.Background(), companyID, []byte("meta-access-token"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := store.Decrypt(context.Background(), companyID, envelope)
	if err != nil || string(plaintext) != "meta-access-token" {
		t.Fatalf("decrypt=%q err=%v", plaintext, err)
	}
	if _, err = store.Decrypt(context.Background(), uuid.New(), envelope); err == nil {
		t.Fatal("another tenant decrypted the credential")
	}
}
