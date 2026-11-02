package push

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	optionKey = "push_config"

	// SecretMask is returned to the UI instead of the stored API key. Saving it back keeps the old key.
	SecretMask = "********"
)

// Config holds the OneSignal settings, stored as JSON in bm_options.
type Config struct {
	Enabled    bool   `json:"enabled"`
	AppId      string `json:"app_id"`
	RestApiKey string `json:"rest_api_key"`
	DefaultUrl string `json:"default_url"` // opened on click when a notification has no url
}

// LoadConfig reads the push config. A missing config returns a disabled default.
func LoadConfig(ctx context.Context) (*Config, error) {
	cfg := &Config{}

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

// SaveConfig stores the push config. A masked API key keeps its previously stored value.
func SaveConfig(ctx context.Context, cfg *Config) error {
	if cfg.RestApiKey == SecretMask {
		old, err := LoadConfig(ctx)
		if err != nil {
			return err
		}
		cfg.RestApiKey = old.RestApiKey
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
	if cp.RestApiKey != "" {
		cp.RestApiKey = SecretMask
	}
	return &cp
}
