package push

import (
	v1 "billionmail-core/api/push/v1"
	"billionmail-core/internal/service/push"
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
)

func (c *ControllerV1) Send(ctx context.Context, req *v1.SendReq) (res *v1.SendRes, err error) {
	res = &v1.SendRes{}

	n := push.Notification{Title: req.Title, Message: req.Message, Url: req.Url, Image: req.Image}

	if req.Audience == push.AudienceAll {
		err = push.SendToAll(ctx, n)
	} else {
		res.Data.Targeted, err = push.SendToGroup(ctx, req.GroupId, n)
	}
	if err != nil {
		res.SetError(gerror.Newf("Push failed: %v", err))
		return res, nil
	}

	res.SetSuccess("Push notification sent")
	return
}
