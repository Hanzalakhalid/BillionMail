package sms

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	ProviderTwilio  = "twilio"
	ProviderGeneric = "generic"

	optionKey = "sms_config"

	// SecretMask is returned to the UI instead of stored secrets. Saving it back keeps the old secret.
	SecretMask = "********"
)

// Config holds the SMS provider settings, stored as JSON in bm_options.
type Config struct {
	Enabled            bool   `json:"enabled"`
	Provider           string `json:"provider"`             // twilio | generic
	DefaultCountryCode string `json:"default_country_code"` // e.g. "92" turns 0300... into +92300...

	// Twilio
	TwilioAccountSid string `json:"twilio_account_sid"`
	TwilioAuthToken  string `json:"twilio_auth_token"`
	TwilioFrom       string `json:"twilio_from"`

	// Generic HTTP gateway (most local bulk SMS providers).
	// Placeholders: {to} (+923001234567), {to_digits} (923001234567), {message}, {api_key}, {sender}
	GenericMethod      string `json:"generic_method"` // GET | POST
	GenericURL         string `json:"generic_url"`
	GenericBody        string `json:"generic_body"`         // POST body template
	GenericContentType string `json:"generic_content_type"` // POST content type
	GenericApiKey      string `json:"generic_api_key"`
	GenericSender      string `json:"generic_sender"`
}

// LoadConfig reads the SMS config. A missing config returns a disabled default.
func LoadConfig(ctx context.Context) (*Config, error) {
	cfg := &Config{Provider: ProviderGeneric, GenericMethod: "GET", DefaultCountryCode: "92"}

	val, err := g.DB().Model("bm_options").Where("name", optionKey).Value("value")
	if err != nil {
		return nil, err
	}
	if val.IsEmpty() {
		return cfg, nil
	}
	if err = json.Unmarshal([]byte(val.String()), cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveConfig stores the SMS config. Masked secrets keep their previously stored value.
func SaveConfig(ctx context.Context, cfg *Config) error {
	if cfg.Provider != ProviderTwilio && cfg.Provider != ProviderGeneric {
		return errors.New("unsupported sms provider")
	}

	old, err := LoadConfig(ctx)
	if err != nil {
		return err
	}
	if cfg.TwilioAuthToken == SecretMask {
		cfg.TwilioAuthToken = old.TwilioAuthToken
	}
	if cfg.GenericApiKey == SecretMask {
		cfg.GenericApiKey = old.GenericApiKey
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = g.DB().Model("bm_options").
		Data(g.Map{"name": optionKey, "value": string(data)}).
		OnConflict("name").
		Save()
	return err
}

// Masked returns a copy of the config that is safe to send to the UI.
func (c *Config) Masked() *Config {
	cp := *c
	if cp.TwilioAuthToken != "" {
		cp.TwilioAuthToken = SecretMask
	}
	if cp.GenericApiKey != "" {
		cp.GenericApiKey = SecretMask
	}
	return &cp
}
