package lead_scoring

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func signalNames(r ScoreResult) []string {
	names := make([]string, 0, len(r.Signals))
	for _, s := range r.Signals {
		names = append(names, s.Name)
	}
	return names
}

func TestScoreListing_HiddenGemNoWebsiteIsTier1(t *testing.T) {
	// 4.6 stars, 30 reviews, no website, info@ email, phone
	r := ScoreListing(ListingData{Rating: 4.6, ReviewCount: 30, Email: "info@cafe.com", Phone: "+966500000000"})

	assert.Equal(t, 80, r.Score) // 30 + 25 + 15 + 10
	assert.Equal(t, TagTier1, r.Tag)
	assert.ElementsMatch(t, []string{"hidden_gem", "no_website", "has_email", "has_phone"}, signalNames(r))
}

func TestScoreListing_PopularBusinessIsSkipped(t *testing.T) {
	// Already doing well: many reviews, website, shared inbox
	r := ScoreListing(ListingData{Rating: 4.5, ReviewCount: 500, HasWebsite: true, Email: "info@cafe.com", Phone: "1"})

	assert.Equal(t, 35, r.Score) // 10 + 15 + 10
	assert.Equal(t, 3, r.Tier)
	assert.Equal(t, TagTier3, r.ToAttribs()[AttrLeadTier])
}

func TestScoreListing_LowRatingWithOwnerEmailIsTier2(t *testing.T) {
	r := ScoreListing(ListingData{Rating: 3.8, ReviewCount: 120, HasWebsite: true, Email: "ahmed@cafe.com", Phone: "1"})

	assert.Equal(t, 60, r.Score) // 15 + 10 + 15 + 10 + 10
	assert.Equal(t, TagTier2, r.Tag)
}

func TestScoreListing_NoReviewsNewBusiness(t *testing.T) {
	r := ScoreListing(ListingData{ReviewCount: 0, Email: "owner@new.com"})

	assert.Contains(t, signalNames(r), "no_reviews")
	assert.NotContains(t, signalNames(r), "hidden_gem")
	assert.NotContains(t, signalNames(r), "low_rating", "unknown rating is not a low rating")
}

func TestScoreListing_NoContactNoReach(t *testing.T) {
	r := ScoreListing(ListingData{Rating: 4.2, ReviewCount: 60, HasWebsite: true})

	assert.Equal(t, 0, r.Score)
	assert.Empty(t, r.Signals)
}

func TestIsGenericEmail(t *testing.T) {
	assert.True(t, IsGenericEmail("info@cafe.com"))
	assert.True(t, IsGenericEmail(" Sales@Cafe.com "))
	assert.True(t, IsGenericEmail("connect@cafe.com"))
	assert.False(t, IsGenericEmail("ahmed@cafe.com"))
	assert.False(t, IsGenericEmail("not-an-email"))
}
