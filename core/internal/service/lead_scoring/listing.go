package lead_scoring

import (
	"fmt"
	"strings"
)

// ListingData is what a public Google Maps listing (e.g. the scraper kit CSV) says about a business.
type ListingData struct {
	Rating      float64 // 0 when unknown
	ReviewCount int
	HasWebsite  bool
	Email       string
	Phone       string
}

// genericMailboxes are shared inboxes; any other local part is treated as a person's email.
var genericMailboxes = map[string]bool{
	"info": true, "contact": true, "hello": true, "hi": true, "support": true, "sales": true,
	"admin": true, "office": true, "enquiries": true, "enquiry": true, "inquiries": true,
	"inquiry": true, "reservations": true, "booking": true, "bookings": true, "team": true,
	"mail": true, "help": true, "service": true, "marketing": true, "noreply": true,
	"connect": true, "customercare": true, "care": true, "accounts": true, "hr": true, "careers": true,
}

// ScoreListing rates how much a business needs (and can be reached for) marketing help,
// using only public listing data. Tiers use the same thresholds as Score.
func ScoreListing(d ListingData) ScoreResult {
	var matched []Signal
	total := 0

	check := func(cond bool, s Signal) {
		if cond {
			matched = append(matched, s)
			total += s.Points
		}
	}

	// Need for marketing
	check(d.Rating >= 4.0 && d.ReviewCount > 0 && d.ReviewCount < 50,
		Signal{"hidden_gem", fmt.Sprintf("Good rating (%.1f) but only %d reviews", d.Rating, d.ReviewCount), 30})
	check(d.ReviewCount == 0, Signal{"no_reviews", "No reviews yet (new or unknown business)", 20})
	check(!d.HasWebsite, Signal{"no_website", "No website", 25})
	check(d.Rating > 0 && d.Rating < 4.0, Signal{"low_rating", fmt.Sprintf("Rating %.1f is below 4.0", d.Rating), 15})

	// Can afford it
	check(d.ReviewCount >= 100, Signal{"established", fmt.Sprintf("%d reviews, established business", d.ReviewCount), 10})

	// Can be reached
	email := strings.TrimSpace(d.Email)
	check(email != "", Signal{"has_email", "Email found", 15})
	check(email != "" && !IsGenericEmail(email), Signal{"owner_email", "Personal email (not info@)", 10})
	check(strings.TrimSpace(d.Phone) != "", Signal{"has_phone", "Phone found (SMS/WhatsApp)", 10})

	return newResult(total, matched)
}

// IsGenericEmail reports whether an address is a shared inbox such as info@ or sales@.
func IsGenericEmail(email string) bool {
	local, _, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	return ok && genericMailboxes[local]
}
