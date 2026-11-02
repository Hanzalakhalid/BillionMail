package sms

import (
	v1 "billionmail-core/api/sms/v1"
	"billionmail-core/internal/service/sms"
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) GetConfig(ctx context.Context, req *v1.GetConfigReq) (res *v1.GetConfigRes, err error) {
	res = &v1.GetConfigRes{}

	cfg, err := sms.LoadConfig(ctx)
	if err != nil {
		g.Log().Error(ctx, "load sms config: %v", err)
		res.SetError(gerror.New("Failed to load SMS settings"))
		return res, nil
	}

	res.Data = cfg.Masked()
	res.SetSuccess("Success")
	return
}

func (c *ControllerV1) SaveConfig(ctx context.Context, req *v1.SaveConfigReq) (res *v1.SaveConfigRes, err error) {
	res = &v1.SaveConfigRes{}

	if err = sms.SaveConfig(ctx, &req.Config); err != nil {
		res.SetError(gerror.Newf("Failed to save SMS settings: %v", err))
		return res, nil
	}

	res.SetSuccess("SMS settings saved")
	return
}
