package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// graphBaseURL is a variable so tests can point it at a local server.
var graphBaseURL = "https://graph.facebook.com"

// Template identifies an approved WhatsApp message template and its body values.
type Template struct {
	Name     string
	Language string   // e.g. en, en_US, ur
	Params   []string // values for {{1}}, {{2}}, ... in the template body
}

// TemplateInfo describes one template returned by the WhatsApp Business Account.
type TemplateInfo struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	Status   string `json:"status"`
	Category string `json:"category"`
	Body     string `json:"body"`
}

// SendText sends a customer-service-window text message using credentials that
// were resolved server-side for the owning company. Tokens never reach the UI.
func SendText(ctx context.Context, phoneNumberID, accessToken, apiVersion, to, text string) (string, error) {
	if strings.TrimSpace(phoneNumberID) == "" || strings.TrimSpace(accessToken) == "" || strings.TrimSpace(to) == "" {
		return "", errors.New("WhatsApp phone number, token and recipient are required")
	}
	if strings.TrimSpace(text) == "" || len(text) > 4096 {
		return "", errors.New("WhatsApp text must contain 1 to 4096 characters")
	}
	if apiVersion == "" {
		apiVersion = DefaultApiVersion
	}
	payload, err := json.Marshal(map[string]any{
		"messaging_product": "whatsapp", "recipient_type": "individual", "to": strings.TrimPrefix(to, "+"),
		"type": "text", "text": map[string]any{"preview_url": false, "body": text},
	})
	if err != nil {
		return "", err
	}
	cfg := &Config{PhoneNumberId: phoneNumberID, AccessToken: accessToken, ApiVersion: apiVersion}
	endpoint := fmt.Sprintf("%s/%s/%s/messages", graphBaseURL, cfg.ApiVersion, url.PathEscape(cfg.PhoneNumberId))
	raw, err := doGraph(ctx, cfg, http.MethodPost, endpoint, payload)
	if err != nil {
		return "", err
	}
	var out struct {
		Messages []struct {
			Id string `json:"id"`
		} `json:"messages"`
	}
	if err = json.Unmarshal(raw, &out); err != nil || len(out.Messages) == 0 {
		return "", fmt.Errorf("unexpected whatsapp response: %s", snippet(raw))
	}
	return out.Messages[0].Id, nil
}

type graphError struct {
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// sendTemplate sends one template message. to must be digits with country code (923001234567).
// Returns the WhatsApp message id.
func sendTemplate(ctx context.Context, cfg *Config, to string, tpl Template) (string, error) {
	if cfg.PhoneNumberId == "" || cfg.AccessToken == "" {
		return "", errors.New("whatsapp phone number id and access token are required")
	}
	if tpl.Name == "" {
		return "", errors.New("template name is required")
	}

	template := map[string]any{
		"name":     tpl.Name,
		"language": map[string]string{"code": tpl.Language},
	}
	if len(tpl.Params) > 0 {
		params := make([]map[string]string, 0, len(tpl.Params))
		for _, p := range tpl.Params {
			params = append(params, map[string]string{"type": "text", "text": p})
		}
		template["components"] = []map[string]any{{"type": "body", "parameters": params}}
	}

	payload, err := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "template",
		"template":          template,
	})
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/%s/%s/messages", graphBaseURL, cfg.ApiVersion, url.PathEscape(cfg.PhoneNumberId))
	raw, err := doGraph(ctx, cfg, http.MethodPost, endpoint, payload)
	if err != nil {
		return "", err
	}

	var out struct {
		Messages []struct {
			Id string `json:"id"`
		} `json:"messages"`
	}
	if err = json.Unmarshal(raw, &out); err != nil || len(out.Messages) == 0 {
		return "", fmt.Errorf("unexpected whatsapp response: %s", snippet(raw))
	}
	return out.Messages[0].Id, nil
}

// listTemplates returns the templates of the configured WhatsApp Business Account.
func listTemplates(ctx context.Context, cfg *Config) ([]TemplateInfo, error) {
	if cfg.BusinessAccountId == "" || cfg.AccessToken == "" {
		return nil, errors.New("whatsapp business account id and access token are required")
	}

	endpoint := fmt.Sprintf("%s/%s/%s/message_templates?fields=name,language,status,category,components&limit=200",
		graphBaseURL, cfg.ApiVersion, url.PathEscape(cfg.BusinessAccountId))
	raw, err := doGraph(ctx, cfg, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var out struct {
		Data []struct {
			Name       string `json:"name"`
			Language   string `json:"language"`
			Status     string `json:"status"`
			Category   string `json:"category"`
			Components []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"components"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unexpected whatsapp response: %s", snippet(raw))
	}

	list := make([]TemplateInfo, 0, len(out.Data))
	for _, t := range out.Data {
		info := TemplateInfo{Name: t.Name, Language: t.Language, Status: t.Status, Category: t.Category}
		for _, c := range t.Components {
			if strings.EqualFold(c.Type, "BODY") {
				info.Body = c.Text
			}
		}
		list = append(list, info)
	}
	return list, nil
}

func doGraph(ctx context.Context, cfg *Config, method, endpoint string, payload []byte) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ge graphError
		if json.Unmarshal(raw, &ge) == nil && ge.Error != nil && ge.Error.Message != "" {
			return nil, fmt.Errorf("whatsapp error %d: %s", ge.Error.Code, ge.Error.Message)
		}
		return nil, fmt.Errorf("whatsapp returned HTTP %d: %s", resp.StatusCode, snippet(raw))
	}
	return raw, nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}
