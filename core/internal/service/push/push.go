package push

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	StatusSent   = "sent"
	StatusFailed = "failed"

	AudienceAll = "all"
)

// SendToAll pushes n to every subscriber of the OneSignal app and records it in bm_push_logs.
func SendToAll(ctx context.Context, n Notification) error {
	cfg, err := enabledConfig(ctx)
	if err != nil {
		return err
	}

	id, err := sendOneSignal(ctx, cfg, n, nil)
	writeLog(ctx, n, AudienceAll, 0, id, err)
	return err
}

// SendToGroup pushes n to subscribed contacts of a group. A contact receives it only if the
// website identified that subscriber with their email (OneSignal external_id = contact email).
// Returns how many contacts were targeted.
func SendToGroup(ctx context.Context, groupId int, n Notification) (int, error) {
	cfg, err := enabledConfig(ctx)
	if err != nil {
		return 0, err
	}

	emails, err := g.DB().Model("bm_contacts").
		Where("group_id", groupId).
		Where("active", 1).
		Array("email")
	if err != nil {
		return 0, err
	}
	if len(emails) == 0 {
		return 0, errors.New("no subscribed contacts in this group")
	}

	ids := make([]string, 0, len(emails))
	for _, e := range emails {
		ids = append(ids, strings.ToLower(e.String()))
	}

	audience := fmt.Sprintf("group:%d", groupId)
	var notificationIds []string
	var sendErr error
	for _, batch := range chunk(ids, maxExternalIds) {
		id, err := sendOneSignal(ctx, cfg, n, batch)
		if err != nil {
			sendErr = err
			continue
		}
		notificationIds = append(notificationIds, id)
	}

	// A batch with no reachable subscribers is not a failure if another batch got through.
	if len(notificationIds) > 0 {
		sendErr = nil
	}
	writeLog(ctx, n, audience, len(ids), strings.Join(notificationIds, ","), sendErr)
	return len(ids), sendErr
}

func enabledConfig(ctx context.Context) (*Config, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, errors.New("push notifications are disabled, enable them in Push settings first")
	}
	return cfg, nil
}

func chunk(items []string, size int) [][]string {
	var out [][]string
	for size < len(items) {
		items, out = items[size:], append(out, items[:size])
	}
	return append(out, items)
}

func writeLog(ctx context.Context, n Notification, audience string, targeted int, notificationId string, sendErr error) {
	status, errMsg := StatusSent, ""
	if sendErr != nil {
		status, errMsg = StatusFailed, sendErr.Error()
	}

	_, err := g.DB().Model("bm_push_logs").Data(g.Map{
		"title":           n.Title,
		"message":         n.Message,
		"url":             n.Url,
		"audience":        audience,
		"targeted":        targeted,
		"notification_id": notificationId,
		"status":          status,
		"error":           errMsg,
		"create_time":     time.Now().Unix(),
	}).Insert()
	if err != nil {
		g.Log().Error(ctx, "[Push] failed to write log:", err)
	}
}
