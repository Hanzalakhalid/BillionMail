package sms

import (
	v1 "billionmail-core/api/sms/v1"
	"billionmail-core/internal/service/sms"
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
)

func (c *ControllerV1) SendTest(ctx context.Context, req *v1.SendTestReq) (res *v1.SendTestRes, err error) {
	res = &v1.SendTestRes{}

	if err = sms.SendOne(ctx, req.Phone, req.Message, 0); err != nil {
		res.SetError(gerror.Newf("SMS failed: %v", err))
		return res, nil
	}

	res.SetSuccess("SMS sent")
	return
}

func (c *ControllerV1) SendGroup(ctx context.Context, req *v1.SendGroupReq) (res *v1.SendGroupRes, err error) {
	res = &v1.SendGroupRes{}

	queued, err := sms.SendToGroup(ctx, req.GroupId, req.Message)
	if err != nil {
		res.SetError(gerror.Newf("SMS failed: %v", err))
		return res, nil
	}

	res.Data.Queued = queued
	res.SetSuccess("SMS sending started")
	return
}
