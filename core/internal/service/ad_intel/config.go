package ad_intel

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	optionKey = "ad_intel_config"

	// SecretMask is returned to the UI instead of the stored token. Saving it back keeps the old token.
	SecretMask = "********"

	DefaultResultsLimit = 100
	MaxResultsLimit     = 500
)

// Config holds the Ad Finder settings, stored as JSON in bm_options.
type Config struct {
	ApifyToken     string `json:"apify_token"`
	DefaultCountry string `json:"default_country"` // ISO code used by the Ad Library, e.g. PK, SA, AE
	ResultsLimit   int    `json:"results_limit"`   // ads fetched per search (Apify bills per ad)
}

// LoadConfig reads the config, filling defaults.
func LoadConfig(ctx context.Context) (*Config, error) {
	cfg := &Config{DefaultCountry: "PK", ResultsLimit: DefaultResultsLimit}

	val, err := g.DB().Model("bm_options").Where("name", optionKey).Value("value")
	if err != nil {
		return nil, err
	}
	if !val.IsEmpty() {
		if err = json.Unmarshal([]byte(val.String()), cfg); err != nil {
			return nil, err
		}
	}
	cfg.ResultsLimit = clampLimit(cfg.ResultsLimit)
	return cfg, nil
}

// SaveConfig stores the config. A masked token keeps its stored value.
func SaveConfig(ctx context.Context, cfg *Config) error {
	if cfg.ApifyToken == SecretMask {
		old, err := LoadConfig(ctx)
		if err != nil {
			return err
		}
		cfg.ApifyToken = old.ApifyToken
	}
	cfg.ResultsLimit = clampLimit(cfg.ResultsLimit)

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
	if cp.ApifyToken != "" {
		cp.ApifyToken = SecretMask
	}
	return &cp
}

func clampLimit(n int) int {
	if n <= 0 {
		return DefaultResultsLimit
	}
	if n > MaxResultsLimit {
		return MaxResultsLimit
	}
	return n
}
