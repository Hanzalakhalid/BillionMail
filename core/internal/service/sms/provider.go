package sms

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// twilioBaseURL is a variable so tests can point it at a local server.
var twilioBaseURL = "https://api.twilio.com"

// NormalizePhone turns a user-entered number into E.164 (+923001234567).
// Local numbers starting with a single 0 get the default country code.
func NormalizePhone(phone, defaultCountryCode string) (string, error) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(phone) {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r == '+' && i == 0 {
			b.WriteRune(r)
		}
	}
	p := b.String()

	switch {
	case strings.HasPrefix(p, "+"):
	case strings.HasPrefix(p, "00"):
		p = "+" + p[2:]
	case strings.HasPrefix(p, "0"):
		if defaultCountryCode == "" {
			return "", fmt.Errorf("local number %q needs a default country code", phone)
		}
		p = "+" + strings.TrimPrefix(defaultCountryCode, "+") + p[1:]
	default:
		p = "+" + p
	}

	digits := len(p) - 1
	if digits < 8 || digits > 15 {
		return "", fmt.Errorf("invalid phone number %q", phone)
	}
	return p, nil
}

// send delivers one message through the configured provider. phone must be E.164.
func send(ctx context.Context, cfg *Config, phone, message string) error {
	switch cfg.Provider {
	case ProviderTwilio:
		return sendTwilio(ctx, cfg, phone, message)
	case ProviderGeneric:
		return sendGeneric(ctx, cfg, phone, message)
	}
	return errors.New("unsupported sms provider")
}

func sendTwilio(ctx context.Context, cfg *Config, phone, message string) error {
	if cfg.TwilioAccountSid == "" || cfg.TwilioAuthToken == "" || cfg.TwilioFrom == "" {
		return errors.New("twilio account sid, auth token and from number are required")
	}

	form := url.Values{}
	form.Set("To", phone)
	form.Set("From", cfg.TwilioFrom)
	form.Set("Body", message)

	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", twilioBaseURL, url.PathEscape(cfg.TwilioAccountSid))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfg.TwilioAccountSid, cfg.TwilioAuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return doRequest(req)
}

func sendGeneric(ctx context.Context, cfg *Config, phone, message string) error {
	if cfg.GenericURL == "" {
		return errors.New("gateway url is required")
	}

	values := map[string]string{
		"{to}":        phone,
		"{to_digits}": strings.TrimPrefix(phone, "+"),
		"{message}":   message,
		"{api_key}":   cfg.GenericApiKey,
		"{sender}":    cfg.GenericSender,
	}

	method := strings.ToUpper(cfg.GenericMethod)
	if method == "" {
		method = http.MethodGet
	}

	endpoint := fillTemplate(cfg.GenericURL, values, url.QueryEscape)

	var body io.Reader
	if method == http.MethodPost && cfg.GenericBody != "" {
		escape := func(s string) string { return s }
		if strings.Contains(cfg.GenericContentType, "json") {
			escape = jsonEscape
		} else if strings.Contains(cfg.GenericContentType, "x-www-form-urlencoded") {
			escape = url.QueryEscape
		}
		body = strings.NewReader(fillTemplate(cfg.GenericBody, values, escape))
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if body != nil && cfg.GenericContentType != "" {
		req.Header.Set("Content-Type", cfg.GenericContentType)
	}

	return doRequest(req)
}

func fillTemplate(tpl string, values map[string]string, escape func(string) string) string {
	for k, v := range values {
		tpl = strings.ReplaceAll(tpl, k, escape(v))
	}
	return tpl
}

// jsonEscape escapes a value for use inside a JSON string literal.
func jsonEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return r.Replace(s)
}

func doRequest(req *http.Request) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return fmt.Errorf("provider returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}
