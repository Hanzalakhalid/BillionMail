package video_outreach

import (
	v1 "billionmail-core/api/video_outreach/v1"
	vo "billionmail-core/internal/service/video_outreach"
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) ImportLeads(ctx context.Context, req *v1.ImportLeadsReq) (res *v1.ImportLeadsRes, err error) {
	res = &v1.ImportLeadsRes{}

	count, err := g.DB().Model("bm_contact_groups").Where("id", req.GroupId).Count()
	if err != nil || count == 0 {
		res.SetError(gerror.New("Contact group not found"))
		return res, nil
	}

	leads, skipped, err := vo.ParseListingCSV(req.Content)
	if err != nil {
		res.SetError(gerror.Newf("Invalid CSV: %v", err))
		return res, nil
	}

	now := time.Now().Unix()
	for _, lead := range leads {
		attribs, _ := json.Marshal(lead.Attribs)
		// Re-importing keeps other attributes and refreshes the score
		_, err := g.DB().Exec(ctx, `INSERT INTO bm_contacts (email, group_id, active, status, attribs, create_time)
			VALUES (?, ?, 1, 0, CAST(? AS jsonb), ?)
			ON CONFLICT (group_id, email) DO UPDATE SET attribs = COALESCE(bm_contacts.attribs, '{}'::jsonb) || EXCLUDED.attribs`,
			lead.Email, req.GroupId, string(attribs), now)
		if err != nil {
			g.Log().Warningf(ctx, "import lead %s: %v", lead.Email, err)
			skipped++
			continue
		}

		res.Data.Imported++
		switch lead.Result.Tier {
		case 1:
			res.Data.Tier1++
		case 2:
			res.Data.Tier2++
		default:
			res.Data.Tier3++
		}
	}

	res.Data.Skipped = skipped
	res.SetSuccess("Leads imported and scored")
	return
}
