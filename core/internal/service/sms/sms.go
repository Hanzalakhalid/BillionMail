package sms

import (
	"context"
	"errors"
	"regexp"
	"time"

	"billionmail-core/internal/model/entity"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	StatusSent   = "sent"
	StatusFailed = "failed"

	// AttrPhone is the contact attribute that holds the phone number.
	AttrPhone = "phone"

	// sendInterval spaces out group sends to stay under provider rate limits.
	sendInterval = 200 * time.Millisecond
)

var placeholderRe = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

// Personalize replaces {key} with contact attributes ({email} is the contact email).
// Unknown placeholders are left empty.
func Personalize(message string, contact entity.Contact) string {
	return placeholderRe.ReplaceAllStringFunc(message, func(m string) string {
		key := m[1 : len(m)-1]
		if key == "email" {
			return contact.Email
		}
		return contact.Attribs[key]
	})
}

// SendOne sends a single SMS and records it in bm_sms_logs.
func SendOne(ctx context.Context, phone, message string, groupId int) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return errors.New("sms sending is disabled, enable it in SMS settings first")
	}
	return sendAndLog(ctx, cfg, phone, message, groupId)
}

func sendAndLog(ctx context.Context, cfg *Config, phone, message string, groupId int) error {
	normalized, err := NormalizePhone(phone, cfg.DefaultCountryCode)
	if err == nil {
		err = send(ctx, cfg, normalized, message)
	} else {
		normalized = phone
	}

	status, errMsg := StatusSent, ""
	if err != nil {
		status, errMsg = StatusFailed, err.Error()
	}

	_, logErr := g.DB().Model("bm_sms_logs").Data(g.Map{
		"phone":       normalized,
		"message":     message,
		"provider":    cfg.Provider,
		"status":      status,
		"error":       errMsg,
		"group_id":    groupId,
		"create_time": time.Now().Unix(),
	}).Insert()
	if logErr != nil {
		g.Log().Error(ctx, "[SMS] failed to write log:", logErr)
	}
	return err
}

// SendToGroup queues an SMS to every subscribed contact in the group that has a phone attribute.
// Sending runs in the background; it returns how many contacts will be messaged.
func SendToGroup(ctx context.Context, groupId int, message string) (int, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return 0, err
	}
	if !cfg.Enabled {
		return 0, errors.New("sms sending is disabled, enable it in SMS settings first")
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
			if err := sendAndLog(bgCtx, cfg, c.Attribs[AttrPhone], Personalize(message, c), groupId); err != nil {
				g.Log().Warningf(bgCtx, "[SMS] send to contact %d failed: %v", c.Id, err)
			}
		}
	}()

	return len(contacts), nil
}
