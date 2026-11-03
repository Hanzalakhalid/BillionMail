package ad_intel

import (
	"billionmail-core/internal/service/lead_scoring"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SearchResult summarizes one Ad Library search.
type SearchResult struct {
	Ads         int `json:"ads"`
	Advertisers int `json:"advertisers"`
}

// Search fetches active ads for a keyword and saves the advertisers it finds.
func Search(ctx context.Context, keyword, country string, limit int) (*SearchResult, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("enter what to search for, e.g. dental Karachi")
	}
	if country == "" {
		country = cfg.DefaultCountry
	}
	country = strings.ToUpper(country)
	if limit <= 0 {
		limit = cfg.ResultsLimit
	}

	items, err := fetchAds(ctx, cfg, keyword, country, limit)
	if err != nil {
		return nil, err
	}

	advertisers := Aggregate(items, time.Now())
	for _, a := range advertisers {
		if err := saveAdvertiser(ctx, a, keyword, country); err != nil {
			g.Log().Warningf(ctx, "[AdFinder] save advertiser %s: %v", a.PageId, err)
		}
	}
	return &SearchResult{Ads: len(items), Advertisers: len(advertisers)}, nil
}

// saveAdvertiser inserts or refreshes an advertiser; the lead it may already be is kept.
func saveAdvertiser(ctx context.Context, a Advertiser, keyword, country string) error {
	now := time.Now().Unix()
	_, err := g.DB().Exec(ctx, `INSERT INTO bm_ad_advertisers (page_id, page_name, page_url, website, category, ad_topic, ad_text, cta,
			ad_count, days_running, platforms, likes, score, reasons, keyword, country, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (page_id) DO UPDATE SET page_name = EXCLUDED.page_name, page_url = EXCLUDED.page_url,
			website = CASE WHEN EXCLUDED.website <> '' THEN EXCLUDED.website ELSE bm_ad_advertisers.website END,
			category = EXCLUDED.category, ad_topic = EXCLUDED.ad_topic, ad_text = EXCLUDED.ad_text, cta = EXCLUDED.cta,
			ad_count = EXCLUDED.ad_count, days_running = EXCLUDED.days_running, platforms = EXCLUDED.platforms,
			likes = EXCLUDED.likes, score = EXCLUDED.score, reasons = EXCLUDED.reasons,
			keyword = EXCLUDED.keyword, country = EXCLUDED.country, update_time = EXCLUDED.update_time`,
		a.PageId, a.PageName, a.PageUrl, a.Website, a.Category, a.AdTopic, a.AdText, a.Cta,
		a.AdCount, a.DaysRunning, strings.Join(a.Platforms, ","), a.Likes, a.Score, strings.Join(a.Reasons, "; "),
		keyword, country, now, now)
	return err
}

// AddToLeads turns an advertiser into a contact in a group, ready for Video Outreach.
// Without an email, one is looked up on the advertiser's website.
func AddToLeads(ctx context.Context, pageId string, groupId int, email string) (string, error) {
	row, err := g.DB().Model("bm_ad_advertisers").Where("page_id", pageId).One()
	if err != nil {
		return "", err
	}
	if row.IsEmpty() {
		return "", errors.New("advertiser not found")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		if email, err = FindEmail(ctx, row["website"].String()); err != nil {
			return "", errors.New("no email found on their website, enter one from their Facebook page")
		}
	}

	score := row["score"].Int()
	tier := lead_scoring.TagTier3
	if score >= lead_scoring.Tier1Threshold {
		tier = lead_scoring.TagTier1
	} else if score >= lead_scoring.Tier2Threshold {
		tier = lead_scoring.TagTier2
	}

	attribs := map[string]string{
		"name":         row["page_name"].String(),
		"website":      row["website"].String(),
		"fb_page":      row["page_url"].String(),
		"category":     row["category"].String(),
		"ad_topic":     row["ad_topic"].String(),
		"ad_days":      strconv.Itoa(row["days_running"].Int()),
		"ad_count":     strconv.Itoa(row["ad_count"].Int()),
		"lead_score":   strconv.Itoa(score),
		"lead_tier":    tier,
		"lead_reasons": row["reasons"].String(),
		"source":       "facebook_ad_library",
	}
	for k, v := range attribs {
		if v == "" {
			delete(attribs, k)
		}
	}
	data, _ := json.Marshal(attribs)

	_, err = g.DB().Exec(ctx, `INSERT INTO bm_contacts (email, group_id, active, status, attribs, create_time)
		VALUES (?, ?, 1, 0, CAST(? AS jsonb), ?)
		ON CONFLICT (group_id, email) DO UPDATE SET attribs = COALESCE(bm_contacts.attribs, '{}'::jsonb) || EXCLUDED.attribs`,
		email, groupId, string(data), time.Now().Unix())
	if err != nil {
		return "", err
	}

	_, err = g.DB().Model("bm_ad_advertisers").Where("page_id", pageId).
		Data(g.Map{"lead_email": email, "lead_group_id": groupId}).Update()
	return email, err
}
