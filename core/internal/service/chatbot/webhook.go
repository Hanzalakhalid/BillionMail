package chatbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	platformqueue "billionmail-core/internal/service/queue"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

type webhookTask struct {
	WebhookEventID uuid.UUID `json:"webhook_event_id"`
}
type metaEnvelope struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Contacts []struct {
					WAID    string `json:"wa_id"`
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
				} `json:"contacts"`
				Messages []struct {
					ID   string `json:"id"`
					From string `json:"from"`
					Type string `json:"type"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
					Interactive struct {
						Type        string `json:"type"`
						ButtonReply struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"button_reply"`
						ListReply struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"list_reply"`
					} `json:"interactive"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// HandleWebhook is the durable worker entry point for an accepted Meta event.
func HandleWebhook(ctx context.Context, event platformqueue.Event) error {
	var task webhookTask
	if err := json.Unmarshal(event.Payload, &task); err != nil || task.WebhookEventID == uuid.Nil {
		return errors.New("invalid webhook task")
	}
	return tenantTx(ctx, event.CompanyID, func(tx gdb.TX) error {
		var stored struct {
			ID            uuid.UUID       `json:"id"`
			PhoneNumberID uuid.UUID       `json:"phone_number_id"`
			Payload       json.RawMessage `json:"payload"`
			Status        string          `json:"status"`
		}
		if err := tx.Model("webhook_events").Fields("id,phone_number_id,payload,status").Where("id", task.WebhookEventID).Where("company_id", event.CompanyID).LockUpdate().Scan(&stored); err != nil {
			return err
		}
		if stored.ID == uuid.Nil {
			return errors.New("webhook event not found")
		}
		if stored.Status == "processed" {
			return nil
		}
		var envelope metaEnvelope
		if err := json.Unmarshal(stored.Payload, &envelope); err != nil {
			return fmt.Errorf("decode Meta event: %w", err)
		}
		if _, err := tx.Model("webhook_events").Data(g.Map{"status": "processing", "attempts": gdb.Raw("attempts+1")}).Where("id", stored.ID).Update(); err != nil {
			return err
		}
		for _, entry := range envelope.Entry {
			for _, change := range entry.Changes {
				profileNames := map[string]string{}
				for _, contact := range change.Value.Contacts {
					profileNames[contact.WAID] = contact.Profile.Name
				}
				for _, message := range change.Value.Messages {
					input := strings.TrimSpace(message.Text.Body)
					if message.Type == "interactive" {
						if message.Interactive.ButtonReply.ID != "" {
							input = message.Interactive.ButtonReply.ID
						} else {
							input = message.Interactive.ListReply.ID
						}
					}
					if message.ID == "" || message.From == "" || input == "" {
						continue
					}
					if err := processInbound(tx, event.CompanyID, stored.PhoneNumberID, message.ID, message.From, profileNames[message.From], message.Type, input, message); err != nil {
						return err
					}
				}
			}
		}
		_, err := tx.Model("webhook_events").Data(g.Map{"status": "processed", "processed_at": time.Now().UTC(), "last_error": nil}).Where("id", stored.ID).Update()
		return err
	})
}

func processInbound(tx gdb.TX, companyID, phoneID uuid.UUID, providerMessageID, from, profileName, messageType, input string, raw any) error {
	now := time.Now().UTC()
	contactID, err := returningID(tx, `INSERT INTO whatsapp_contacts(id,company_id,phone_number_id,wa_id,profile_name,created_at,updated_at)
        VALUES(?,?,?,?,?,?,?) ON CONFLICT(company_id,phone_number_id,wa_id) DO UPDATE
        SET profile_name=COALESCE(NULLIF(EXCLUDED.profile_name,''),whatsapp_contacts.profile_name),updated_at=EXCLUDED.updated_at RETURNING id`,
		uuid.New(), companyID, phoneID, from, profileName, now, now)
	if err != nil {
		return err
	}
	conversationID, err := returningID(tx, `INSERT INTO whatsapp_conversations(id,company_id,phone_number_id,contact_id,status,automation_mode,last_customer_message_at,last_message_at,created_at,updated_at)
        VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(company_id,phone_number_id,contact_id) DO UPDATE
        SET status='open',last_customer_message_at=EXCLUDED.last_customer_message_at,last_message_at=EXCLUDED.last_message_at,updated_at=EXCLUDED.updated_at RETURNING id`,
		uuid.New(), companyID, phoneID, contactID, "open", "bot", now, now, now, now)
	if err != nil {
		return err
	}
	rawJSON, _ := json.Marshal(raw)
	result, err := tx.Exec(`INSERT INTO whatsapp_messages(id,company_id,conversation_id,provider_message_id,direction,message_type,body,payload,status,created_at)
        VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(company_id,provider_message_id) DO NOTHING`,
		uuid.New(), companyID, conversationID, providerMessageID, "inbound", messageType, input, rawJSON, "received", now)
	if err != nil {
		return err
	}
	inserted, _ := result.RowsAffected()
	if inserted == 0 {
		return nil
	}
	var conversation struct {
		AutomationMode string `json:"automation_mode"`
	}
	if err = tx.Model("whatsapp_conversations").Fields("automation_mode").Where("id", conversationID).Scan(&conversation); err != nil {
		return err
	}
	if conversation.AutomationMode != "bot" {
		return nil
	}

	var state struct {
		FlowID        uuid.UUID `json:"flow_id"`
		FlowVersionID uuid.UUID `json:"flow_version_id"`
		CurrentNodeID string    `json:"current_node_id"`
		Status        string    `json:"status"`
	}
	_ = tx.Model("chatbot_conversation_states").Fields("flow_id,flow_version_id,current_node_id,status").Where("conversation_id", conversationID).Scan(&state)
	var selected struct {
		FlowID    uuid.UUID       `json:"flow_id"`
		VersionID uuid.UUID       `json:"version_id"`
		Graph     json.RawMessage `json:"graph"`
	}
	if state.FlowVersionID != uuid.Nil && state.Status == "waiting" {
		selected.FlowID, selected.VersionID = state.FlowID, state.FlowVersionID
		if err = tx.Model("chatbot_flow_versions").Fields("graph").Where("id", state.FlowVersionID).Where("company_id", companyID).Scan(&selected); err != nil {
			return err
		}
	} else {
		if err = tx.Model("chatbot_flows f").InnerJoin("chatbot_flow_versions v", "v.id=f.active_version_id AND v.company_id=f.company_id").
			Fields("f.id flow_id,v.id version_id,v.graph").Where("f.company_id", companyID).Where("f.phone_number_id", phoneID).Where("f.status", "active").WhereNull("f.deleted_at").Scan(&selected); err != nil {
			return err
		}
	}
	if selected.FlowID == uuid.Nil || len(selected.Graph) == 0 {
		return nil
	}
	var graph Graph
	if err = json.Unmarshal(selected.Graph, &graph); err != nil {
		return err
	}
	current := ""
	if state.Status == "waiting" {
		current = state.CurrentNodeID
	}
	engineResult, err := Execute(graph, current, input)
	if err != nil {
		return err
	}
	for _, body := range engineResult.Messages {
		messageID := uuid.New()
		if _, err = tx.Model("whatsapp_messages").Data(g.Map{"id": messageID, "company_id": companyID, "conversation_id": conversationID, "direction": "outbound", "message_type": "text", "body": body, "status": "queued"}).Insert(); err != nil {
			return err
		}
		payload, _ := json.Marshal(g.Map{"message_id": messageID, "phone_number_id": phoneID, "to": from, "type": "text", "text": body})
		if _, err = tx.Model("outbox_events").Data(g.Map{"id": uuid.New(), "company_id": companyID, "aggregate_type": "whatsapp_message", "aggregate_id": messageID.String(), "event_type": "whatsapp.message.send", "payload": payload, "idempotency_key": messageID.String()}).Insert(); err != nil {
			return err
		}
	}
	status := "completed"
	if engineResult.Waiting {
		status = "waiting"
	}
	_, err = tx.Exec(`INSERT INTO chatbot_conversation_states(id,company_id,conversation_id,flow_id,flow_version_id,current_node_id,status,updated_at,created_at)
        VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(conversation_id) DO UPDATE SET flow_id=EXCLUDED.flow_id,flow_version_id=EXCLUDED.flow_version_id,
        current_node_id=EXCLUDED.current_node_id,status=EXCLUDED.status,updated_at=EXCLUDED.updated_at`,
		uuid.New(), companyID, conversationID, selected.FlowID, selected.VersionID, engineResult.CurrentNode, status, now, now)
	return err
}

func returningID(tx gdb.TX, query string, args ...any) (uuid.UUID, error) {
	var value struct {
		ID uuid.UUID `json:"id"`
	}
	if err := tx.GetStruct(&value, query, args...); err != nil {
		return uuid.Nil, err
	}
	return value.ID, nil
}
