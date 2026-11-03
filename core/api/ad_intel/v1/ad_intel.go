package v1

import (
	"billionmail-core/internal/service/ad_intel"
	"billionmail-core/utility/types/api_v1"

	"github.com/gogf/gf/v2/frame/g"
)

type GetConfigReq struct {
	g.Meta        `path:"/ad_intel/config" method:"get" tags:"AdFinder" summary:"Get Ad Finder settings"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
}

type GetConfigRes struct {
	api_v1.StandardRes
	Data *ad_intel.Config `json:"data"`
}

type SaveConfigReq struct {
	g.Meta        `path:"/ad_intel/config" method:"post" tags:"AdFinder" summary:"Save Ad Finder settings"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	ad_intel.Config
}

type SaveConfigRes struct {
	api_v1.StandardRes
}

type SearchReq struct {
	g.Meta        `path:"/ad_intel/search" method:"post" tags:"AdFinder" summary:"Find businesses running ads for a keyword"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Keyword       string `json:"keyword" v:"required|max-length:100" dc:"What to search, e.g. dental Karachi"`
	Country       string `json:"country" v:"max-length:3" dc:"Ad Library country code, e.g. PK, SA, AE, ALL"`
	Limit         int    `json:"limit" v:"min:0|max:500" dc:"Ads to fetch (Apify bills per ad)"`
}

type SearchRes struct {
	api_v1.StandardRes
	Data *ad_intel.SearchResult `json:"data"`
}

type ListReq struct {
	g.Meta        `path:"/ad_intel/list" method:"get" tags:"AdFinder" summary:"List advertisers found, best first"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Keyword       string `json:"keyword" dc:"Only advertisers found by this search" in:"query"`
	Search        string `json:"search" dc:"Search name, website or ad text" in:"query"`
	Page          int    `json:"page" v:"min:1" dc:"Page number" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" dc:"Page size" in:"query" default:"20"`
}

type AdvertiserRow struct {
	PageId       string `json:"page_id"`
	PageName     string `json:"page_name"`
	PageUrl      string `json:"page_url"`
	AdLibraryUrl string `json:"ad_library_url"`
	Website      string `json:"website"`
	Category     string `json:"category"`
	AdTopic      string `json:"ad_topic"`
	AdText       string `json:"ad_text"`
	Cta          string `json:"cta"`
	AdCount      int    `json:"ad_count"`
	DaysRunning  int    `json:"days_running"`
	Platforms    string `json:"platforms"`
	Score        int    `json:"score"`
	Reasons      string `json:"reasons"`
	Keyword      string `json:"keyword"`
	Country      string `json:"country"`
	LeadEmail    string `json:"lead_email"`
	UpdateTime   int    `json:"update_time"`
}

type ListRes struct {
	api_v1.StandardRes
	Data struct {
		Total    int              `json:"total"`
		List     []*AdvertiserRow `json:"list"`
		Keywords []string         `json:"keywords" dc:"Searches done so far"`
	} `json:"data"`
}

type AddLeadReq struct {
	g.Meta        `path:"/ad_intel/add_lead" method:"post" tags:"AdFinder" summary:"Add an advertiser to a contact group as a lead"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	PageId        string `json:"page_id" v:"required" dc:"Advertiser page ID"`
	GroupId       int    `json:"group_id" v:"required|min:1" dc:"Contact group"`
	Email         string `json:"email" v:"email" dc:"Optional; found on their website when empty"`
}

type AddLeadRes struct {
	api_v1.StandardRes
	Data struct {
		Email string `json:"email"`
	} `json:"data"`
}
