package video_outreach

import (
	v1 "billionmail-core/api/video_outreach/v1"
	"billionmail-core/internal/model/entity"
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) ListVideoOutreach(ctx context.Context, req *v1.ListVideoOutreachReq) (res *v1.ListVideoOutreachRes, err error) {
	res = &v1.ListVideoOutreachRes{}

	q := g.DB().Model("bm_contacts")

	if req.GroupID > 0 {
		q = q.Where("group_id", req.GroupID)
	}

	// Filter by tier via the JSONB attribs (->> avoids the ? operator, which GoFrame treats as a placeholder)
	if req.Tier != "" {
		q = q.Where("attribs->>'lead_tier' = ?", req.Tier)
	} else {
		// Only show contacts that have lead scoring data
		q = q.Where("COALESCE(attribs->>'lead_tier', '') <> ''")
	}

	// Count total
	total, err := q.Count()
	if err != nil {
		g.Log().Error(ctx, "count video outreach contacts: %v", err)
		total = 0
	}

	// Fetch page, best leads first
	var contacts []entity.Contact
	err = q.Order(gdb.Raw("NULLIF(regexp_replace(attribs->>'lead_score', '[^0-9]', '', 'g'), '')::int DESC NULLS LAST, create_time DESC")).
		Page(req.Page, req.PageSize).
		Scan(&contacts)
	if err != nil {
		g.Log().Error(ctx, "list video outreach contacts: %v", err)
		res.SetSuccess("Listed video outreach contacts")
		return
	}

	list := make([]*v1.VideoStatus, 0, len(contacts))
	for _, c := range contacts {
		list = append(list, toVideoStatus(c))
	}

	res.Data.Total = total
	res.Data.List = list
	res.SetSuccess("Listed video outreach contacts")
	return
}
