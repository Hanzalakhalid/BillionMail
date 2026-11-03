package ad_intel

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Advertiser is every active ad of one Facebook page, summarized.
type Advertiser struct {
	PageId      string   `json:"page_id"`
	PageName    string   `json:"page_name"`
	PageUrl     string   `json:"page_url"`
	Website     string   `json:"website"`
	Category    string   `json:"category"`
	AdTopic     string   `json:"ad_topic"` // short "what the ad is about", for personal messages
	AdText      string   `json:"ad_text"`
	Cta         string   `json:"cta"`
	AdCount     int      `json:"ad_count"`
	DaysRunning int      `json:"days_running"` // age of the oldest active ad
	Platforms   []string `json:"platforms"`
	Likes       int      `json:"likes"`
	Score       int      `json:"score"`
	Reasons     []string `json:"reasons"`
}

// Hosts that are not the advertiser's own website.
var socialHosts = []string{"facebook.com", "fb.me", "fb.com", "instagram.com", "wa.me", "whatsapp.com", "m.me", "messenger.com", "threads.net"}

// Aggregate groups ads by page and scores each advertiser, best first.
func Aggregate(items []AdItem, now time.Time) []Advertiser {
	byPage := map[string]*Advertiser{}
	oldest := map[string]int64{}
	platforms := map[string]map[string]bool{}
	var order []string

	for _, it := range items {
		if it.PageID == "" {
			continue
		}
		a, ok := byPage[it.PageID]
		if !ok {
			a = &Advertiser{PageId: it.PageID, PageName: it.PageName}
			byPage[it.PageID] = a
			platforms[it.PageID] = map[string]bool{}
			order = append(order, it.PageID)
		}

		count := it.CollationCount
		if count < 1 {
			count = 1
		}
		a.AdCount += count
		for _, p := range it.PublisherPlatform {
			platforms[it.PageID][strings.ToUpper(p)] = true
		}

		s := it.Snapshot
		if a.PageUrl == "" {
			a.PageUrl = s.PageProfileUri
		}
		if a.Category == "" && len(s.PageCategories) > 0 {
			a.Category = s.PageCategories[0]
		}
		if s.PageLikeCount > a.Likes {
			a.Likes = s.PageLikeCount
		}
		if a.Website == "" {
			a.Website = websiteOf(s.LinkUrl)
		}

		// The longest-running ad is the one that works, so it describes the advertiser
		if it.StartDate > 0 && (oldest[it.PageID] == 0 || it.StartDate < oldest[it.PageID]) {
			oldest[it.PageID] = it.StartDate
			if topic := adTopic(it); topic != "" {
				a.AdTopic = topic
				a.AdText = strings.TrimSpace(s.Body.Text)
				a.Cta = s.CtaText
			}
		} else if a.AdTopic == "" {
			a.AdTopic = adTopic(it)
			a.AdText = strings.TrimSpace(s.Body.Text)
			a.Cta = s.CtaText
		}
	}

	result := make([]Advertiser, 0, len(order))
	for _, id := range order {
		a := byPage[id]
		if start := oldest[id]; start > 0 {
			a.DaysRunning = int(now.Sub(time.Unix(start, 0)).Hours() / 24)
		}
		for p := range platforms[id] {
			a.Platforms = append(a.Platforms, p)
		}
		sort.Strings(a.Platforms)
		a.Score, a.Reasons = Score(*a)
		result = append(result, *a)
	}

	sort.SliceStable(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	return result
}

// Score rates how much budget an advertiser is putting into ads (0-100).
func Score(a Advertiser) (int, []string) {
	score, reasons := 30, []string{"Running Facebook/Instagram ads now"}

	switch {
	case a.AdCount >= 5:
		score += 25
		reasons = append(reasons, fmt.Sprintf("%d active ads (big budget)", a.AdCount))
	case a.AdCount >= 2:
		score += 10
		reasons = append(reasons, fmt.Sprintf("%d active ads", a.AdCount))
	}

	switch {
	case a.DaysRunning >= 60:
		score += 25
		reasons = append(reasons, fmt.Sprintf("Ad running %d days (it works, people respond)", a.DaysRunning))
	case a.DaysRunning >= 30:
		score += 10
		reasons = append(reasons, fmt.Sprintf("Ad running %d days", a.DaysRunning))
	}

	if len(a.Platforms) >= 2 {
		score += 10
		reasons = append(reasons, "Advertises on "+strings.Join(a.Platforms, ", "))
	}
	if a.Website != "" {
		score += 10
		reasons = append(reasons, "Has a website (email can be found)")
	}
	return score, reasons
}

// adTopic picks a short description of what the ad sells.
func adTopic(it AdItem) string {
	for _, s := range []string{it.Snapshot.Title, it.Snapshot.Body.Text} {
		s = strings.Join(strings.Fields(s), " ")
		if s == "" || strings.Contains(s, "{{") {
			continue
		}
		if r := []rune(s); len(r) > 90 {
			s = strings.TrimSpace(string(r[:90])) + "..."
		}
		return s
	}
	return ""
}

// websiteOf returns scheme://host of an ad's link, unless it points to a social network.
func websiteOf(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	for _, social := range socialHosts {
		if host == social || strings.HasSuffix(host, "."+social) {
			return ""
		}
	}
	return u.Scheme + "://" + u.Host
}
