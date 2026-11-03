package video_outreach

import (
	v1 "billionmail-core/api/video_outreach/v1"
	vo "billionmail-core/internal/service/video_outreach"
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) GetStats(ctx context.Context, req *v1.GetStatsReq) (res *v1.GetStatsRes, err error) {
	res = &v1.GetStatsRes{}

	groupFilter, args := "", []interface{}{}
	if req.GroupID > 0 {
		groupFilter, args = " AND c.group_id = ?", append(args, req.GroupID)
	}

	ready, err := g.DB().GetValue(ctx, `SELECT COUNT(*) FROM bm_contacts c
		WHERE COALESCE(c.attribs->>'`+vo.AttrVideoURL+`', '') <> ''`+groupFilter, args...)
	if err != nil {
		g.Log().Error(ctx, "count ready videos: %v", err)
	}

	row, err := g.DB().GetOne(ctx, `SELECT
			COUNT(*) FILTER (WHERE e.event = 'view') AS views,
			COUNT(*) FILTER (WHERE e.event = 'play') AS plays,
			COUNT(*) FILTER (WHERE e.event = 'cta') AS clicks,
			COUNT(DISTINCT e.contact_id) FILTER (WHERE e.event = 'view') AS leads_viewed,
			COUNT(DISTINCT e.contact_id) FILTER (WHERE e.event = 'cta') AS leads_clicked
		FROM bm_video_events e JOIN bm_contacts c ON c.id = e.contact_id
		WHERE TRUE`+groupFilter, args...)
	if err != nil {
		g.Log().Error(ctx, "video outreach stats: %v", err)
	}

	res.Data.ReadyVideos = ready.Int()
	res.Data.Views = row["views"].Int()
	res.Data.Plays = row["plays"].Int()
	res.Data.Clicks = row["clicks"].Int()
	res.Data.LeadsViewed = row["leads_viewed"].Int()
	res.Data.LeadsClicked = row["leads_clicked"].Int()
	res.SetSuccess("Success")
	return res, nil
}

// eventCounts returns per-contact landing page activity for the given contacts.
func eventCounts(ctx context.Context, contactIDs []int) map[int]*v1.VideoStatus {
	counts := make(map[int]*v1.VideoStatus)
	if len(contactIDs) == 0 {
		return counts
	}

	rows, err := g.DB().Model("bm_video_events").
		Fields("contact_id",
			"COUNT(*) FILTER (WHERE event = 'view') AS views",
			"COUNT(*) FILTER (WHERE event = 'play') AS plays",
			"COUNT(*) FILTER (WHERE event = 'cta') AS clicks",
			"MAX(create_time) AS last_activity").
		WhereIn("contact_id", contactIDs).
		Group("contact_id").
		All()
	if err != nil {
		g.Log().Error(ctx, "video event counts: %v", err)
		return counts
	}

	for _, r := range rows {
		counts[r["contact_id"].Int()] = &v1.VideoStatus{
			Views:        r["views"].Int(),
			Plays:        r["plays"].Int(),
			Clicks:       r["clicks"].Int(),
			LastActivity: r["last_activity"].Int(),
		}
	}
	return counts
}
