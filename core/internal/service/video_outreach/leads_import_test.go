package video_outreach

import (
	"billionmail-core/internal/service/lead_scoring"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Same columns as the scraper kit's *-clean.csv export, including the UTF-8 BOM.
var scraperCSV = utf8BOM + "\"Priority\",\"ProspectType\",\"Company\",\"Category\",\"Phone\",\"Email\",\"Website\",\"Address\",\"Rating\",\"ReviewCount\",\"ContactCoverage\",\"WhyFit\",\"GoogleMaps\",\"EmailStatus\"\n" +
	"\"1\",\"x\",\"Cafe Al Noor\",\"Cafe\",\"+966500000001\",\"info@alnoor.com\",\"\",\"Riyadh\",\"4.6\",\"30\",\"\",\"\",\"https://maps.google.com/?cid=1\",\"\"\n" +
	"\"2\",\"x\",\"Big Chain\",\"Cafe\",\"+966500000002\",\"info@chain.com\",\"https://chain.com\",\"Riyadh\",\"4.5\",\"1,200\",\"\",\"\",\"\",\"\"\n" +
	"\"3\",\"x\",\"No Email Cafe\",\"Cafe\",\"+966500000003\",\"\",\"\",\"Riyadh\",\"4.9\",\"10\",\"\",\"\",\"\",\"\"\n" +
	"\"4\",\"x\",\"Duplicate\",\"Cafe\",\"\",\"INFO@alnoor.com\",\"\",\"Riyadh\",\"4.0\",\"5\",\"\",\"\",\"\",\"\"\n"

func TestParseListingCSV_ScraperFormat(t *testing.T) {
	leads, skipped, err := ParseListingCSV(scraperCSV)

	require.NoError(t, err)
	assert.Equal(t, 2, skipped, "row without email and duplicate email are skipped")
	require.Len(t, leads, 2)

	gem := leads[0]
	assert.Equal(t, "info@alnoor.com", gem.Email)
	assert.Equal(t, lead_scoring.TagTier1, gem.Attribs[lead_scoring.AttrLeadTier])
	assert.Equal(t, "80", gem.Attribs[lead_scoring.AttrLeadScore])
	assert.Equal(t, "Cafe Al Noor", gem.Attribs["name"])
	assert.Equal(t, "+966500000001", gem.Attribs["phone"])
	assert.Equal(t, "https://maps.google.com/?cid=1", gem.Attribs["maps_url"])
	assert.Contains(t, gem.Attribs[AttrLeadReasons], "No website")
	assert.NotContains(t, gem.Attribs, "website", "empty cells are not stored")

	chain := leads[1]
	assert.Equal(t, "1,200", chain.Attribs["review_count"])
	assert.Equal(t, lead_scoring.TagTier3, chain.Attribs[lead_scoring.AttrLeadTier])
}

func TestParseListingCSV_RawScrapeColumns(t *testing.T) {
	// Columns written by the scraper kit's scripts/scrape.sh
	csv := "title,phone,emails,website,category,address,review_rating,review_count\n" +
		"Cafe Noor,+92300,info@noor.pk,,Cafe,Karachi,4.7,20\n"

	leads, _, err := ParseListingCSV(csv)

	require.NoError(t, err)
	require.Len(t, leads, 1)
	assert.Equal(t, "Cafe Noor", leads[0].Attribs["name"])
	assert.Equal(t, "4.7", leads[0].Attribs["rating"])
	assert.Equal(t, lead_scoring.TagTier1, leads[0].Attribs[lead_scoring.AttrLeadTier])
}

func TestParseListingCSV_MultipleEmailsInCell(t *testing.T) {
	leads, _, err := ParseListingCSV("name,email\nCafe,\"not-an-email; Owner@Cafe.com\"\n")

	require.NoError(t, err)
	require.Len(t, leads, 1)
	assert.Equal(t, "owner@cafe.com", leads[0].Email)
}

func TestParseListingCSV_RequiresEmailColumn(t *testing.T) {
	_, _, err := ParseListingCSV("name,phone\nCafe,123\n")
	assert.Error(t, err)
}

func TestBuildLandingURL(t *testing.T) {
	u := BuildLandingURL("https://mail.example.com/", "abc123")

	parsed, err := url.Parse(u)
	require.NoError(t, err)
	assert.Equal(t, "mail.example.com", parsed.Host)
	assert.Equal(t, "/landing/video", parsed.Path)
	assert.Equal(t, "abc123", parsed.Query().Get("t"))
}

func TestNewVideoToken_UniqueAndURLSafe(t *testing.T) {
	a, b := NewVideoToken(), NewVideoToken()

	assert.Len(t, a, 24)
	assert.NotEqual(t, a, b)
	assert.Equal(t, a, url.QueryEscape(a))
}

func TestIsValidEvent(t *testing.T) {
	assert.True(t, IsValidEvent(EventView))
	assert.True(t, IsValidEvent(EventPlay))
	assert.True(t, IsValidEvent(EventCTA))
	assert.False(t, IsValidEvent("delete"))
	assert.False(t, IsValidEvent(""))
}
