package whatsappchannel

import (
	"context"
	"errors"
	"testing"
	"time"

	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/tenancy"
	"github.com/google/uuid"
)

type rejectingSecretStore struct{ calls int }

func (s *rejectingSecretStore) Encrypt(context.Context, uuid.UUID, []byte) (secrets.Envelope, error) {
	s.calls++
	return secrets.Envelope{}, errors.New("unexpected encryption")
}

func (*rejectingSecretStore) Decrypt(context.Context, uuid.UUID, secrets.Envelope) ([]byte, error) {
	return nil, errors.New("unexpected decryption")
}

func TestConnectRejectsInvalidOrUnauthorizedInputBeforeEncryption(t *testing.T) {
	owner := tenancy.TenantContext{UserID: uuid.New(), CompanyID: uuid.New(), Role: "owner"}
	valid := ConnectInput{WABAID: "waba-1", AccessToken: "secret", PhoneNumbers: []PhoneAsset{{PhoneNumberID: "phone-1"}}}

	tests := []struct {
		name   string
		tenant tenancy.TenantContext
		input  ConnectInput
	}{
		{name: "missing tenant", input: valid},
		{name: "agent cannot manage channels", tenant: tenancy.TenantContext{UserID: uuid.New(), CompanyID: uuid.New(), Role: "agent"}, input: valid},
		{name: "missing token", tenant: owner, input: ConnectInput{WABAID: "waba-1", PhoneNumbers: valid.PhoneNumbers}},
		{name: "missing phone", tenant: owner, input: ConnectInput{WABAID: "waba-1", AccessToken: "secret"}},
		{name: "duplicate phone", tenant: owner, input: ConnectInput{WABAID: "waba-1", AccessToken: "secret", PhoneNumbers: []PhoneAsset{{PhoneNumberID: "same"}, {PhoneNumberID: "same"}}}},
		{name: "expired token", tenant: owner, input: ConnectInput{WABAID: "waba-1", AccessToken: "secret", TokenExpiresAt: timePointer(time.Now().Add(-time.Minute)), PhoneNumbers: valid.PhoneNumbers}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &rejectingSecretStore{}
			if _, err := Connect(context.Background(), test.tenant, store, test.input); err == nil {
				t.Fatal("Connect accepted invalid input")
			}
			if store.calls != 0 {
				t.Fatal("invalid request reached secret encryption")
			}
		})
	}
}

func TestChannelConflictRedactsDatabaseDetails(t *testing.T) {
	err := channelConflict(errors.New(`duplicate key value violates unique constraint "whatsapp_accounts_provider_waba_id_key"`))
	if !errors.Is(err, ErrChannelUnavailable) {
		t.Fatalf("conflict = %v", err)
	}
}

func timePointer(value time.Time) *time.Time { return &value }
