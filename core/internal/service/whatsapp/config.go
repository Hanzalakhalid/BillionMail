package whatsapp

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	optionKey = "whatsapp_config"

	// SecretMask is returned to the UI instead of the stored token. Saving it back keeps the old token.
	SecretMask = "********"

	// DefaultApiVersion is the Graph API version used when none is configured.
	DefaultApiVersion = "v23.0"
)

// Config holds the WhatsApp Cloud API settings, stored as JSON in bm_options.
type Config struct {
	Enabled            bool   `json:"enabled"`
	PhoneNumberId      string `json:"phone_number_id"`
	BusinessAccountId  string `json:"business_account_id"` // optional, used to list approved templates
	AccessToken        string `json:"access_token"`
	ApiVersion         string `json:"api_version"`
	DefaultCountryCode string `json:"default_country_code"` // e.g. "92" turns 0300... into 92300...
}

// LoadConfig reads the WhatsApp config. A missing config returns a disabled default.
func LoadConfig(ctx context.Context) (*Config, error) {
	cfg := &Config{ApiVersion: DefaultApiVersion, DefaultCountryCode: "92"}

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
	if cfg.ApiVersion == "" {
		cfg.ApiVersion = DefaultApiVersion
	}
	return cfg, nil
}

// SaveConfig stores the WhatsApp config. A masked token keeps its previously stored value.
func SaveConfig(ctx context.Context, cfg *Config) error {
	if cfg.AccessToken == SecretMask {
		old, err := LoadConfig(ctx)
		if err != nil {
			return err
		}
		cfg.AccessToken = old.AccessToken
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
	if cp.AccessToken != "" {
		cp.AccessToken = SecretMask
	}
	return &cp
}
