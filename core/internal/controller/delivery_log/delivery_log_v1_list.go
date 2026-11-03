package delivery_log

import (
	v1 "billionmail-core/api/delivery_log/v1"
	dl "billionmail-core/internal/service/delivery_log"
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error) {
	res = &v1.ListRes{}

	q := g.DB().Model("mailstat_send_mails m").
		LeftJoin("mailstat_senders s", "s.postfix_message_id = m.postfix_message_id")

	if req.Status != "" {
		q = q.Where("m.status", req.Status)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		q = q.Where("(m.recipient ILIKE ? OR s.sender ILIKE ?)", like, like)
	}
	if req.StartTime > 0 {
		q = q.WhereGTE("m.log_time", req.StartTime)
	}
	if req.EndTime > 0 {
		q = q.WhereLTE("m.log_time", req.EndTime)
	}

	total, err := q.Count()
	if err != nil {
		g.Log().Error(ctx, "count delivery logs: %v", err)
		total = 0
	}

	list := make([]*v1.DeliveryLog, 0)
	err = q.Fields("m.postfix_message_id, m.log_time, m.status, COALESCE(s.sender, '') AS sender, "+
		"m.recipient, m.mail_provider, m.dsn, m.relay, m.delay, m.description").
		OrderDesc("m.log_time_millis").
		Page(req.Page, req.PageSize).
		Scan(&list)
	if err != nil {
		g.Log().Error(ctx, "list delivery logs: %v", err)
	}

	for _, item := range list {
		item.Reason = dl.ExplainDSN(item.Dsn)
	}

	res.Data.Total = total
	res.Data.List = list
	res.SetSuccess("Success")
	return res, nil
}
