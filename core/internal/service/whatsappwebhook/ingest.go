package whatsappwebhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

var (
	ErrInvalidPayload     = errors.New("invalid Meta webhook payload")
	ErrUnknownPhoneNumber = errors.New("unknown WhatsApp phone number")
)

const MaxPayloadBytes = 2 << 20

type envelope struct {
	Object string `json:"object"`
	Entry  []struct {
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				Metadata struct {
					PhoneNumberID string `json:"phone_number_id"`
				} `json:"metadata"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

type IngestResult struct {
	EventID   uuid.UUID
	Duplicate bool
}

func Ingest(ctx context.Context, body []byte, signature string) (IngestResult, error) {
	if len(body) == 0 || len(body) > MaxPayloadBytes || !json.Valid(body) {
		return IngestResult{}, ErrInvalidPayload
	}
	var payload envelope
	if err := json.Unmarshal(body, &payload); err != nil || payload.Object != "whatsapp_business_account" {
		return IngestResult{}, ErrInvalidPayload
	}
	phoneNumberID := envelopePhoneNumberID(payload)
	if phoneNumberID == "" {
		return IngestResult{}, ErrInvalidPayload
	}

	var phone struct {
		ID        uuid.UUID `json:"id"`
		CompanyID uuid.UUID `json:"company_id"`
	}
	if err := g.DB().Model("whatsapp_phone_numbers").Fields("id,company_id").
		Where("provider", "meta").Where("phone_number_id", phoneNumberID).
		Where("status", "active").WhereNull("deleted_at").Scan(&phone); err != nil {
		return IngestResult{}, err
	}
	if phone.ID == uuid.Nil || phone.CompanyID == uuid.Nil {
		return IngestResult{}, ErrUnknownPhoneNumber
	}

	digest := sha256.Sum256(body)
	providerEventID := hex.EncodeToString(digest[:])
	eventID := uuid.New()
	result := IngestResult{EventID: eventID}
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", phone.CompanyID.String()); err != nil {
			return err
		}
		insert, err := tx.Exec(`INSERT INTO webhook_events
            (id,company_id,"provider",provider_event_id,phone_number_id,event_type,payload,"signature",status,received_at)
			VALUES(?,?,?,?,?,?,?,?,?,?)
            ON CONFLICT ("provider", provider_event_id) DO NOTHING`,
			eventID, phone.CompanyID, "meta", providerEventID, phone.ID, "envelope", string(body), signature, "pending", time.Now().UTC())
		if err != nil {
			return err
		}
		rows, err := insert.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			result.Duplicate = true
			return nil
		}
		outboxPayload, _ := json.Marshal(g.Map{"webhook_event_id": eventID, "provider": "meta"})
		_, err = tx.Model("outbox_events").Data(g.Map{
			"id": uuid.New(), "company_id": phone.CompanyID, "aggregate_type": "webhook_event",
			"aggregate_id": eventID.String(), "event_type": "whatsapp.webhook.received",
			"payload": outboxPayload, "idempotency_key": providerEventID,
		}).Insert()
		return err
	})
	if err != nil {
		return IngestResult{}, fmt.Errorf("persist Meta webhook: %w", err)
	}
	return result, nil
}

func envelopePhoneNumberID(payload envelope) string {
	var found string
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" || change.Value.Metadata.PhoneNumberID == "" {
				continue
			}
			if found != "" && found != change.Value.Metadata.PhoneNumberID {
				return ""
			}
			found = change.Value.Metadata.PhoneNumberID
		}
	}
	return found
}
