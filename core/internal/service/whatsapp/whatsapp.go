package whatsapp

import (
	"context"
	"errors"
	"strings"
	"time"

	"billionmail-core/internal/model/entity"
	"billionmail-core/internal/service/sms"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	StatusSent   = "sent"
	StatusFailed = "failed"

	// sendInterval spaces out group sends to stay under WhatsApp rate limits.
	sendInterval = 200 * time.Millisecond
)

// SendOne sends a template to one phone number and records it in bm_whatsapp_logs.
func SendOne(ctx context.Context, phone string, tpl Template) error {
	cfg, err := enabledConfig(ctx)
	if err != nil {
		return err
	}
	return sendAndLog(ctx, cfg, phone, tpl, 0)
}

// SendToGroup queues a template to every subscribed contact in the group that has a phone attribute.
// Template params may use {name}-style contact attributes. Sending runs in the background;
// it returns how many contacts will be messaged.
func SendToGroup(ctx context.Context, groupId int, tpl Template) (int, error) {
	cfg, err := enabledConfig(ctx)
	if err != nil {
		return 0, err
	}

	var contacts []entity.Contact
	err = g.DB().Model("bm_contacts").
		Where("group_id", groupId).
		Where("active", 1).
		Where("COALESCE(attribs->>'phone', '') <> ''").
		Scan(&contacts)
	if err != nil {
		return 0, err
	}
	if len(contacts) == 0 {
		return 0, errors.New("no subscribed contacts with a phone number in this group")
	}

	go func() {
		bgCtx := context.Background()
		for i, c := range contacts {
			if i > 0 {
				time.Sleep(sendInterval)
			}
			personal := tpl
			personal.Params = make([]string, len(tpl.Params))
			for j, p := range tpl.Params {
				personal.Params[j] = sms.Personalize(p, c)
			}
			if err := sendAndLog(bgCtx, cfg, c.Attribs[sms.AttrPhone], personal, groupId); err != nil {
				g.Log().Warningf(bgCtx, "[WhatsApp] send to contact %d failed: %v", c.Id, err)
			}
		}
	}()

	return len(contacts), nil
}

// ListTemplates returns the approved and pending templates of the business account.
func ListTemplates(ctx context.Context) ([]TemplateInfo, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return listTemplates(ctx, cfg)
}

func enabledConfig(ctx context.Context) (*Config, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, errors.New("whatsapp sending is disabled, enable it in WhatsApp settings first")
	}
	return cfg, nil
}

func sendAndLog(ctx context.Context, cfg *Config, phone string, tpl Template, groupId int) error {
	// WhatsApp expects the number without "+"
	to, err := sms.NormalizePhone(phone, cfg.DefaultCountryCode)
	var messageId string
	if err == nil {
		to = strings.TrimPrefix(to, "+")
		messageId, err = sendTemplate(ctx, cfg, to, tpl)
	} else {
		to = phone
	}

	status, errMsg := StatusSent, ""
	if err != nil {
		status, errMsg = StatusFailed, err.Error()
	}

	_, logErr := g.DB().Model("bm_whatsapp_logs").Data(g.Map{
		"phone":       to,
		"template":    tpl.Name,
		"params":      strings.Join(tpl.Params, " | "),
		"message_id":  messageId,
		"status":      status,
		"error":       errMsg,
		"group_id":    groupId,
		"create_time": time.Now().Unix(),
	}).Insert()
	if logErr != nil {
		g.Log().Error(ctx, "[WhatsApp] failed to write log:", logErr)
	}
	return err
}
