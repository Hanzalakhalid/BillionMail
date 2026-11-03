package v1

import (
	"billionmail-core/utility/types/api_v1"

	"github.com/gogf/gf/v2/frame/g"
)

// GenerateVideoReq triggers video generation for a contact.
type GenerateVideoReq struct {
	g.Meta        `path:"/video_outreach/generate" method:"post" tags:"VideoOutreach" summary:"Generate personalized video for a contact"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	ContactEmail  string `json:"contact_email" v:"required|email" dc:"Contact email"`
	GroupID       int    `json:"group_id" v:"required|min:1" dc:"Contact group ID"`
}

type GenerateVideoRes struct {
	api_v1.StandardRes
	Data struct {
		ContactEmail string `json:"contact_email" dc:"Contact email"`
		Status       string `json:"status" dc:"Generation status (queued, processing, completed, failed)"`
	} `json:"data"`
}

// GetVideoStatusReq checks the video generation status for a contact.
type GetVideoStatusReq struct {
	g.Meta        `path:"/video_outreach/status" method:"get" tags:"VideoOutreach" summary:"Get video generation status for a contact"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	ContactEmail  string `json:"contact_email" v:"required|email" dc:"Contact email" in:"query"`
	GroupID       int    `json:"group_id" v:"required|min:1" dc:"Contact group ID" in:"query"`
}

type VideoStatus struct {
	ContactEmail   string `json:"contact_email" dc:"Contact email"`
	GroupId        int    `json:"group_id" dc:"Contact group ID"`
	Name           string `json:"name" dc:"Business or contact name"`
	LeadReasons    string `json:"lead_reasons" dc:"Why the lead got its score"`
	LeadTier       string `json:"lead_tier" dc:"Lead tier (tier_1, tier_2)"`
	LeadScore      string `json:"lead_score" dc:"Lead score"`
	VideoURL       string `json:"video_url" dc:"Video URL (if generated)"`
	ThumbnailURL   string `json:"thumbnail_url" dc:"Thumbnail URL (if generated)"`
	LandingPageURL string `json:"landing_page_url" dc:"Landing page URL (if generated)"`
	Status         string `json:"status" dc:"Status (pending, ready, not_eligible)"`
}

type GetVideoStatusRes struct {
	api_v1.StandardRes
	Data *VideoStatus `json:"data"`
}

// ListVideoOutreachReq lists contacts with video outreach status.
type ListVideoOutreachReq struct {
	g.Meta        `path:"/video_outreach/list" method:"get" tags:"VideoOutreach" summary:"List contacts with video outreach status"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	GroupID       int    `json:"group_id" v:"min:0" dc:"Filter by group ID" in:"query"`
	Tier          string `json:"tier" dc:"Filter by tier (tier_1, tier_2)" in:"query"`
	Page          int    `json:"page" v:"min:1" dc:"Page number" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" dc:"Page size" in:"query" default:"20"`
}

type ListVideoOutreachRes struct {
	api_v1.StandardRes
	Data struct {
		Total int            `json:"total" dc:"Total count"`
		List  []*VideoStatus `json:"list" dc:"Video outreach list"`
	} `json:"data"`
}

// ImportLeadsReq imports a Google Maps listing CSV (e.g. from the scraper kit) and scores every row.
type ImportLeadsReq struct {
	g.Meta        `path:"/video_outreach/import_leads" method:"post" tags:"VideoOutreach" summary:"Import and score leads from a listing CSV"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	GroupId       int    `json:"group_id" v:"required|min:1" dc:"Contact group to import into"`
	Content       string `json:"content" v:"required" dc:"CSV file content"`
}

type ImportLeadsRes struct {
	api_v1.StandardRes
	Data struct {
		Imported int `json:"imported" dc:"Leads added or updated"`
		Skipped  int `json:"skipped" dc:"Rows without a valid email, or duplicates"`
		Tier1    int `json:"tier_1" dc:"Leads in tier 1 (video)"`
		Tier2    int `json:"tier_2" dc:"Leads in tier 2 (text)"`
		Tier3    int `json:"tier_3" dc:"Leads below tier 2 (skip)"`
	} `json:"data"`
}

// SetVideoReq attaches a video recorded outside BillionMail (phone, Loom, ...) to a contact.
type SetVideoReq struct {
	g.Meta        `path:"/video_outreach/set_video" method:"post" tags:"VideoOutreach" summary:"Attach a video link to a contact"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	ContactEmail  string `json:"contact_email" v:"required|email" dc:"Contact email"`
	GroupId       int    `json:"group_id" v:"required|min:1" dc:"Contact group ID"`
	VideoUrl      string `json:"video_url" v:"required|url" dc:"Direct video link (.mp4)"`
	ThumbnailUrl  string `json:"thumbnail_url" v:"url" dc:"Optional preview image"`
	CtaUrl        string `json:"cta_url" v:"url" dc:"Optional Book a Call link"`
}

type SetVideoRes struct {
	api_v1.StandardRes
	Data struct {
		LandingPageUrl string `json:"landing_page_url"`
	} `json:"data"`
}
