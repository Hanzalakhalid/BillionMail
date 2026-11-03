package ad_intel

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

// apifyBaseURL is a variable so tests can point it at a local server.
var apifyBaseURL = "https://api.apify.com"

// actorID is Apify's official Facebook Ad Library scraper.
const actorID = "apify~facebook-ads-scraper"

// The sync endpoint waits for the run to finish; Apify caps it at 300 seconds.
const runTimeout = 290 * time.Second

var httpClient = &http.Client{Timeout: runTimeout + 20*time.Second}

// AdItem is one ad (or group of identical ads) returned by the actor.
type AdItem struct {
	PageID            string   `json:"pageID"`
	PageName          string   `json:"pageName"`
	AdArchiveID       string   `json:"adArchiveID"`
	StartDate         int64    `json:"startDate"`
	IsActive          bool     `json:"isActive"`
	CollationCount    int      `json:"collationCount"`
	PublisherPlatform []string `json:"publisherPlatform"`
	Snapshot          struct {
		PageProfileUri string   `json:"pageProfileUri"`
		CtaText        string   `json:"ctaText"`
		Title          string   `json:"title"`
		LinkUrl        string   `json:"linkUrl"`
		PageCategories []string `json:"pageCategories"`
		PageLikeCount  int      `json:"pageLikeCount"`
		Body           struct {
			Text string `json:"text"`
		} `json:"body"`
	} `json:"snapshot"`
}

// AdLibrarySearchURL is the public Ad Library page for active ads matching a keyword in a country.
func AdLibrarySearchURL(keyword, country string) string {
	q := url.Values{}
	q.Set("active_status", "active")
	q.Set("ad_type", "all")
	q.Set("country", strings.ToUpper(country))
	q.Set("media_type", "all")
	q.Set("q", keyword)
	q.Set("search_type", "keyword_unordered")
	return "https://www.facebook.com/ads/library/?" + q.Encode()
}

// AdLibraryPageURL is the public Ad Library page listing one advertiser's active ads.
func AdLibraryPageURL(pageID, country string) string {
	q := url.Values{}
	q.Set("active_status", "active")
	q.Set("ad_type", "all")
	q.Set("country", strings.ToUpper(country))
	q.Set("view_all_page_id", pageID)
	q.Set("search_type", "page")
	return "https://www.facebook.com/ads/library/?" + q.Encode()
}

// fetchAds runs the Apify actor for a keyword search and returns the ads.
func fetchAds(ctx context.Context, cfg *Config, keyword, country string, limit int) ([]AdItem, error) {
	if cfg.ApifyToken == "" {
		return nil, errors.New("add your Apify API token in Ad Finder settings first")
	}

	payload, err := json.Marshal(map[string]any{
		"startUrls":      []map[string]string{{"url": AdLibrarySearchURL(keyword, country)}},
		"resultsLimit":   clampLimit(limit),
		"activeStatus":   "active",
		"isDetailsPerAd": false,
	})
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/v2/acts/%s/run-sync-get-dataset-items?timeout=%d",
		apifyBaseURL, actorID, int(runTimeout.Seconds()))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ApifyToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &apiErr) == nil && apiErr.Error != nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("apify error (%s): %s", apiErr.Error.Type, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("apify returned HTTP %d: %s", resp.StatusCode, snippet(raw))
	}

	var items []AdItem
	if err = json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("unexpected apify response: %s", snippet(raw))
	}
	return items, nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}
