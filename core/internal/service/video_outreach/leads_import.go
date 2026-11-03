package video_outreach

import (
	"billionmail-core/internal/service/lead_scoring"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// AttrLeadReasons holds the human-readable signal labels, joined with "; ".
const AttrLeadReasons = "lead_reasons"

// Lead is one scored row from a listing CSV, ready to upsert into bm_contacts.
type Lead struct {
	Email   string
	Attribs map[string]string
	Result  lead_scoring.ScoreResult
}

// headerAliases maps normalized CSV headers to the attribute they fill.
var headerAliases = map[string]string{
	"company": "name", "name": "name", "businessname": "name", "title": "name",
	"email": "email", "emails": "email",
	"phone": "phone", "phonenumber": "phone",
	"website": "website", "url": "website",
	"rating": "rating", "avgrating": "rating", "reviewrating": "rating",
	"reviewcount": "review_count", "reviews": "review_count", "reviewscount": "review_count",
	"category":   "category",
	"address":    "address",
	"googlemaps": "maps_url", "mapsurl": "maps_url", "googlemapsurl": "maps_url", "link": "maps_url",
}

// utf8BOM is the byte order mark Excel and the scraper kit put at the start of CSV files
var utf8BOM = string([]byte{0xEF, 0xBB, 0xBF})

var nonAlnum = regexp.MustCompile(`[^a-z0-9]`)

// ParseListingCSV reads a Google Maps listing CSV (e.g. from the scraper kit), scores every row
// and returns the leads. Rows without a valid email are counted in skipped.
func ParseListingCSV(content string) (leads []Lead, skipped int, err error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, utf8BOM)))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	columns := make(map[string]int)
	for i, h := range headers {
		if attr, ok := headerAliases[nonAlnum.ReplaceAllString(strings.ToLower(h), "")]; ok {
			if _, seen := columns[attr]; !seen {
				columns[attr] = i
			}
		}
	}
	if _, ok := columns["email"]; !ok {
		return nil, 0, errors.New("CSV needs an Email column")
	}

	seen := make(map[string]bool)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			skipped++
			continue
		}

		value := func(attr string) string {
			if i, ok := columns[attr]; ok && i < len(record) {
				return strings.TrimSpace(record[i])
			}
			return ""
		}

		email := firstEmail(value("email"))
		if email == "" || seen[email] {
			skipped++
			continue
		}
		seen[email] = true

		rating, _ := strconv.ParseFloat(value("rating"), 64)
		reviews, _ := strconv.Atoi(strings.ReplaceAll(value("review_count"), ",", ""))

		result := lead_scoring.ScoreListing(lead_scoring.ListingData{
			Rating:      rating,
			ReviewCount: reviews,
			HasWebsite:  value("website") != "",
			Email:       email,
			Phone:       value("phone"),
		})

		attribs := result.ToAttribs()
		for _, attr := range []string{"name", "phone", "website", "rating", "review_count", "category", "address", "maps_url"} {
			if v := value(attr); v != "" {
				attribs[attr] = v
			}
		}
		reasons := make([]string, 0, len(result.Signals))
		for _, s := range result.Signals {
			reasons = append(reasons, s.Label)
		}
		attribs[AttrLeadReasons] = strings.Join(reasons, "; ")

		leads = append(leads, Lead{Email: email, Attribs: attribs, Result: result})
	}

	return leads, skipped, nil
}

// firstEmail returns the first valid address in a cell that may hold several ("a@x.com; b@y.com").
func firstEmail(cell string) string {
	for _, part := range strings.FieldsFunc(cell, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		if addr, err := mail.ParseAddress(part); err == nil {
			return strings.ToLower(addr.Address)
		}
	}
	return ""
}

// BuildLandingURL returns the public video landing page URL for a contact.
func BuildLandingURL(baseURL, name, videoURL, thumbURL, ctaURL string) string {
	q := url.Values{}
	q.Set("name", name)
	q.Set("video", videoURL)
	if thumbURL != "" {
		q.Set("thumb", thumbURL)
	}
	if ctaURL != "" {
		q.Set("cta", ctaURL)
	}
	return strings.TrimRight(baseURL, "/") + "/landing/video?" + q.Encode()
}
