package video_outreach

import (
	v1 "billionmail-core/api/video_outreach/v1"
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) ListEvents(ctx context.Context, req *v1.ListEventsReq) (res *v1.ListEventsRes, err error) {
	res = &v1.ListEventsRes{}

	q := g.DB().Model("bm_video_events e").
		InnerJoin("bm_contacts c", "c.id = e.contact_id")
	if req.ContactEmail != "" {
		q = q.Where("c.email", req.ContactEmail)
	}
	if req.GroupID > 0 {
		q = q.Where("c.group_id", req.GroupID)
	}

	total, err := q.Count()
	if err != nil {
		g.Log().Error(ctx, "count video events: %v", err)
		total = 0
	}

	list := make([]*v1.VideoEvent, 0)
	err = q.Fields("e.event, e.create_time, c.email AS contact_email, COALESCE(c.attribs->>'name', '') AS name").
		OrderDesc("e.id").
		Page(req.Page, req.PageSize).
		Scan(&list)
	if err != nil {
		g.Log().Error(ctx, "list video events: %v", err)
	}

	res.Data.Total = total
	res.Data.List = list
	res.SetSuccess("Success")
	return res, nil
}
