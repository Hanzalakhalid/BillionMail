package video_outreach

import (
	v1 "billionmail-core/api/video_outreach/v1"
	"billionmail-core/internal/model/entity"
	vo "billionmail-core/internal/service/video_outreach"
)

// toVideoStatus builds the list/status row for a contact from its attributes.
func toVideoStatus(c entity.Contact) *v1.VideoStatus {
	status := &v1.VideoStatus{
		ContactEmail: c.Email,
		GroupId:      c.GroupId,
		Status:       "not_eligible",
	}
	if c.Attribs == nil {
		return status
	}

	status.Name = c.Attribs["name"]
	status.LeadReasons = c.Attribs[vo.AttrLeadReasons]
	status.LeadTier = c.Attribs["lead_tier"]
	status.LeadScore = c.Attribs["lead_score"]
	status.VideoURL = c.Attribs[vo.AttrVideoURL]
	status.ThumbnailURL = c.Attribs[vo.AttrThumbnailURL]
	status.LandingPageURL = c.Attribs[vo.AttrLandingPageURL]

	if vo.HasVideoAssets(c.Attribs) {
		status.Status = "ready"
	} else if status.LeadTier == "tier_1" || status.LeadTier == "tier_2" {
		status.Status = "pending"
	}
	return status
}
