package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// oneSignalBaseURL is a variable so tests can point it at a local server.
var oneSignalBaseURL = "https://api.onesignal.com"

// allSegments are OneSignal's built-in "everyone" segments, tried in order: apps created
// since 2024 have "Total Subscriptions", older apps "Subscribed Users".
var allSegments = []string{"Total Subscriptions", "Subscribed Users"}

// maxExternalIds is OneSignal's limit of aliases per request.
const maxExternalIds = 2000

// Notification is the content of one push message.
type Notification struct {
	Title   string
	Message string
	Url     string
	Image   string
}

type oneSignalResponse struct {
	Id     string          `json:"id"`
	Errors json.RawMessage `json:"errors"`
}

// sendOneSignal sends n to everyone subscribed (externalIds == nil) or to the given external ids.
// Returns the OneSignal notification id.
func sendOneSignal(ctx context.Context, cfg *Config, n Notification, externalIds []string) (string, error) {
	if cfg.AppId == "" || cfg.RestApiKey == "" {
		return "", errors.New("onesignal app id and rest api key are required")
	}
	if externalIds != nil {
		return postNotification(ctx, cfg, n, func(body map[string]any) {
			body["include_aliases"] = map[string][]string{"external_id": externalIds}
		})
	}

	// An unknown segment name reaches nobody, so fall back to the next name
	var err error
	for _, segment := range allSegments {
		var id string
		id, err = postNotification(ctx, cfg, n, func(body map[string]any) {
			body["included_segments"] = []string{segment}
		})
		if !errors.Is(err, errNoRecipients) {
			return id, err
		}
	}
	return "", err
}

// errNoRecipients wraps OneSignal's "accepted but nobody was reachable" answer.
var errNoRecipients = errors.New("onesignal: no subscribers received the notification")

func postNotification(ctx context.Context, cfg *Config, n Notification, target func(map[string]any)) (string, error) {
	body := map[string]any{
		"app_id":         cfg.AppId,
		"target_channel": "push",
		"contents":       map[string]string{"en": n.Message},
	}
	if n.Title != "" {
		body["headings"] = map[string]string{"en": n.Title}
	}
	if url := n.Url; url != "" || cfg.DefaultUrl != "" {
		if url == "" {
			url = cfg.DefaultUrl
		}
		body["url"] = url
	}
	if n.Image != "" {
		body["chrome_web_image"] = n.Image // web push
		body["big_picture"] = n.Image      // android
		body["ios_attachments"] = map[string]string{"image": n.Image}
	}
	target(body)

	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oneSignalBaseURL+"/notifications?c=push", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Key "+cfg.RestApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("onesignal returned HTTP %d: %s", resp.StatusCode, snippet(raw))
	}

	var out oneSignalResponse
	if err = json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("unexpected onesignal response: %s", snippet(raw))
	}
	// OneSignal answers 200 with an "errors" field and no id when nobody was reachable.
	if out.Id == "" {
		if msg := oneSignalErrors(out.Errors); msg != "" {
			return "", fmt.Errorf("%w (%s)", errNoRecipients, msg)
		}
		return "", errNoRecipients
	}
	return out.Id, nil
}

// oneSignalErrors flattens the "errors" field, which is either a list of strings or an object.
func oneSignalErrors(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return strings.Join(list, "; ")
	}
	return snippet(raw)
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}
