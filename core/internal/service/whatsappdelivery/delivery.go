package whatsappdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/whatsapp"
	"billionmail-core/internal/service/whatsappchannel"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

type Handler struct {
	Store      secrets.EnvelopeStore
	APIVersion string
}
type task struct {
	MessageID     uuid.UUID `json:"message_id"`
	PhoneNumberID uuid.UUID `json:"phone_number_id"`
	To            string    `json:"to"`
	Type          string    `json:"type"`
	Text          string    `json:"text"`
}

func (h Handler) Handle(ctx context.Context, event platformqueue.Event) error {
	if h.Store == nil {
		return errors.New("secret store is required")
	}
	var payload task
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	if payload.MessageID == uuid.Nil || payload.PhoneNumberID == uuid.Nil || payload.To == "" || payload.Type != "text" {
		return errors.New("invalid WhatsApp delivery task")
	}
	var channel struct {
		AccountID       uuid.UUID `json:"account_id"`
		ProviderPhoneID string    `json:"provider_phone_id"`
	}
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
			return err
		}
		return tx.Model("whatsapp_phone_numbers").Fields("whatsapp_account_id account_id,phone_number_id provider_phone_id").Where("id", payload.PhoneNumberID).Where("company_id", event.CompanyID).Where("status", "active").Scan(&channel)
	})
	if err != nil {
		return err
	}
	if channel.AccountID == uuid.Nil {
		return errors.New("active WhatsApp channel not found")
	}
	token, err := whatsappchannel.AccessToken(ctx, event.CompanyID, channel.AccountID, h.Store)
	if err != nil {
		return err
	}
	defer clear(token)
	providerID, sendErr := whatsapp.SendText(ctx, channel.ProviderPhoneID, string(token), h.APIVersion, payload.To, payload.Text)
	updateErr := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
			return err
		}
		data := g.Map{"provider_message_id": providerID, "status": "sent", "sent_at": time.Now().UTC(), "error_message": nil}
		if sendErr != nil {
			data["status"] = "failed"
			data["error_message"] = truncate(sendErr.Error(), 2000)
		}
		_, err := tx.Model("whatsapp_messages").Data(data).Where("id", payload.MessageID).Where("company_id", event.CompanyID).Update()
		return err
	})
	if updateErr != nil {
		return updateErr
	}
	return sendErr
}

func clear(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return value[:max]
	}
	return value
}
