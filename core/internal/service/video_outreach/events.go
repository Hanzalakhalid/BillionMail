package video_outreach

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Landing page events
const (
	EventView = "view" // landing page opened
	EventPlay = "play" // video started
	EventCTA  = "cta"  // Book a Call clicked
)

const (
	// AttrVideoToken identifies a contact on its landing page without exposing the contact ID.
	AttrVideoToken = "video_token"
	// AttrCtaURL is where Book a Call sends the visitor.
	AttrCtaURL = "cta_url"
)

// NewVideoToken returns a random, URL-safe token for a landing page link.
func NewVideoToken() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// IsValidEvent reports whether e is an event the landing page may report.
func IsValidEvent(e string) bool {
	return e == EventView || e == EventPlay || e == EventCTA
}

// RecordEvent stores one landing page event for a contact.
func RecordEvent(ctx context.Context, contactID int, event string) {
	if contactID <= 0 || !IsValidEvent(event) {
		return
	}
	_, err := g.DB().Model("bm_video_events").Data(g.Map{
		"contact_id":  contactID,
		"event":       event,
		"create_time": time.Now().Unix(),
	}).Insert()
	if err != nil {
		g.Log().Warningf(ctx, "[VideoOutreach] record %s event for contact %d: %v", event, contactID, err)
	}
}
