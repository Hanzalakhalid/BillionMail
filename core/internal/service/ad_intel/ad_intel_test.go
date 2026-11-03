package ad_intel

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)

func ad(page, name string, daysAgo, collation int, platforms []string, title, body, link string) AdItem {
	var it AdItem
	it.PageID, it.PageName = page, name
	it.StartDate = now.AddDate(0, 0, -daysAgo).Unix()
	it.CollationCount = collation
	it.IsActive = true
	it.PublisherPlatform = platforms
	it.Snapshot.Title = title
	it.Snapshot.Body.Text = body
	it.Snapshot.LinkUrl = link
	it.Snapshot.PageProfileUri = "https://www.facebook.com/" + page
	it.Snapshot.PageCategories = []string{"Dentist"}
	return it
}

func TestAggregate_GroupsByPageAndScores(t *testing.T) {
	items := []AdItem{
		ad("p1", "Smile Dental", 90, 3, []string{"FACEBOOK"}, "Free checkup this month", "Book now", "https://smiledental.pk/offer?utm=x"),
		ad("p1", "Smile Dental", 10, 2, []string{"INSTAGRAM"}, "", "Whitening 50% off", "https://smiledental.pk"),
		ad("p2", "Tiny Clinic", 5, 1, []string{"FACEBOOK"}, "{{product.name}}", "Braces in Karachi", "https://wa.me/923001234567"),
		ad("", "no page", 1, 1, nil, "x", "", ""),
	}

	got := Aggregate(items, now)

	require.Len(t, got, 2)
	best := got[0]
	assert.Equal(t, "p1", best.PageId)
	assert.Equal(t, 5, best.AdCount, "collation counts add up")
	assert.Equal(t, 90, best.DaysRunning, "age of the oldest ad")
	assert.Equal(t, "Free checkup this month", best.AdTopic, "topic comes from the longest-running ad")
	assert.Equal(t, "https://smiledental.pk", best.Website)
	assert.Equal(t, []string{"FACEBOOK", "INSTAGRAM"}, best.Platforms)
	assert.Equal(t, 100, best.Score) // 30 + 25 + 25 + 10 + 10

	small := got[1]
	assert.Equal(t, "Braces in Karachi", small.AdTopic, "template titles are skipped")
	assert.Equal(t, "", small.Website, "WhatsApp links are not a website")
	assert.Equal(t, 30, small.Score)
}

func TestScore_MiddleBands(t *testing.T) {
	score, reasons := Score(Advertiser{AdCount: 2, DaysRunning: 45, Platforms: []string{"FACEBOOK"}})

	assert.Equal(t, 50, score) // 30 + 10 + 10
	assert.Len(t, reasons, 3)
}

func TestWebsiteOf(t *testing.T) {
	assert.Equal(t, "https://shop.example.pk", websiteOf("https://shop.example.pk/sale?x=1"))
	assert.Equal(t, "", websiteOf("https://l.facebook.com/l.php?u=x"))
	assert.Equal(t, "", websiteOf("https://www.instagram.com/brand"))
	assert.Equal(t, "", websiteOf("javascript:alert(1)"))
}

func TestAdLibraryURLs(t *testing.T) {
	u, err := url.Parse(AdLibrarySearchURL("dental Karachi", "pk"))
	require.NoError(t, err)
	assert.Equal(t, "dental Karachi", u.Query().Get("q"))
	assert.Equal(t, "PK", u.Query().Get("country"))
	assert.Equal(t, "active", u.Query().Get("active_status"))

	p, err := url.Parse(AdLibraryPageURL("123", "sa"))
	require.NoError(t, err)
	assert.Equal(t, "123", p.Query().Get("view_all_page_id"))
	assert.Equal(t, "SA", p.Query().Get("country"))
}

func TestFetchAds_CallsActor(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/acts/apify~facebook-ads-scraper/run-sync-get-dataset-items", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		_, _ = w.Write([]byte(`[{"pageID":"p1","pageName":"Smile","startDate":1700000000,"collationCount":2,
			"publisherPlatform":["FACEBOOK"],"snapshot":{"title":"Free checkup","linkUrl":"https://smile.pk","body":{"text":"hi"}}}]`))
	}))
	defer srv.Close()
	old := apifyBaseURL
	apifyBaseURL = srv.URL
	defer func() { apifyBaseURL = old }()

	items, err := fetchAds(context.Background(), &Config{ApifyToken: "tok"}, "dental", "PK", 50)

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Free checkup", items[0].Snapshot.Title)
	assert.Equal(t, float64(50), body["resultsLimit"])
	startURL := body["startUrls"].([]any)[0].(map[string]any)["url"].(string)
	assert.Contains(t, startURL, "q=dental")
}

func TestFetchAds_Errors(t *testing.T) {
	_, err := fetchAds(context.Background(), &Config{}, "x", "PK", 10)
	assert.Error(t, err, "token is required")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"type":"not-enough-usage-to-run-paid-actor","message":"Not enough credit"}}`))
	}))
	defer srv.Close()
	old := apifyBaseURL
	apifyBaseURL = srv.URL
	defer func() { apifyBaseURL = old }()

	_, err = fetchAds(context.Background(), &Config{ApifyToken: "tok"}, "x", "PK", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough credit")
}

func TestExtractEmails(t *testing.T) {
	html := `<a href="mailto:info@smile.pk">Mail</a> owner&#64;smile.pk logo@2x.png
		<img src="icon@3x.webp"> noreply@sentry.io info@smile.pk test@example.com`

	assert.Equal(t, []string{"info@smile.pk", "owner@smile.pk"}, ExtractEmails(html))
}

func TestFindEmail_PrefersOwnDomainAndBlocksPrivateHosts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/contact" {
			_, _ = w.Write([]byte(`write to hello@smile.pk`))
			return
		}
		_, _ = w.Write([]byte(`agency: support@webagency.com`))
	}))
	defer srv.Close()

	// The test server is on 127.0.0.1, which the client must refuse
	_, err := FindEmail(context.Background(), srv.URL)
	assert.Error(t, err, "loopback websites are not fetched")

	// Pretend smile.pk resolves to a public address that routes to the test server
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	oldLookup, oldDial := lookupIP, websiteClient.Transport
	lookupIP = func(ctx context.Context, network, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}
	websiteClient.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, "127.0.0.1:"+port)
		},
	}
	defer func() { lookupIP, websiteClient.Transport = oldLookup, oldDial }()

	email, err := FindEmail(context.Background(), "http://smile.pk:"+port)
	require.NoError(t, err)
	assert.Equal(t, "hello@smile.pk", email)
}

func TestIsPublicIP(t *testing.T) {
	_, dockerNet, _ := net.ParseCIDR("172.66.1.0/24")
	old := localNetworks
	localNetworks = func() []*net.IPNet { return []*net.IPNet{dockerNet} }
	defer func() { localNetworks = old }()

	assert.False(t, isPublicIP(net.ParseIP("127.0.0.1")))
	assert.False(t, isPublicIP(net.ParseIP("10.1.2.3")))
	assert.False(t, isPublicIP(net.ParseIP("169.254.169.254")))
	assert.False(t, isPublicIP(net.ParseIP("172.66.1.5")), "this server's own Docker network")
	assert.True(t, isPublicIP(net.ParseIP("93.184.216.34")))
}
