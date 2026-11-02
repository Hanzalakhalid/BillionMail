package push

import (
	v1 "billionmail-core/api/push/v1"
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) ListLogs(ctx context.Context, req *v1.ListLogsReq) (res *v1.ListLogsRes, err error) {
	res = &v1.ListLogsRes{}

	q := g.DB().Model("bm_push_logs")
	if req.Status != "" {
		q = q.Where("status", req.Status)
	}

	total, err := q.Count()
	if err != nil {
		g.Log().Error(ctx, "count push logs: %v", err)
		total = 0
	}

	list := make([]*v1.PushLog, 0)
	err = q.OrderDesc("id").Page(req.Page, req.PageSize).Scan(&list)
	if err != nil {
		g.Log().Error(ctx, "list push logs: %v", err)
	}

	res.Data.Total = total
	res.Data.List = list
	res.SetSuccess("Success")
	return res, nil
}
