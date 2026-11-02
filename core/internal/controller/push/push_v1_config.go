package push

import (
	v1 "billionmail-core/api/push/v1"
	"billionmail-core/internal/service/push"
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) GetConfig(ctx context.Context, req *v1.GetConfigReq) (res *v1.GetConfigRes, err error) {
	res = &v1.GetConfigRes{}

	cfg, err := push.LoadConfig(ctx)
	if err != nil {
		g.Log().Error(ctx, "load push config: %v", err)
		res.SetError(gerror.New("Failed to load push settings"))
		return res, nil
	}

	res.Data = cfg.Masked()
	res.SetSuccess("Success")
	return
}

func (c *ControllerV1) SaveConfig(ctx context.Context, req *v1.SaveConfigReq) (res *v1.SaveConfigRes, err error) {
	res = &v1.SaveConfigRes{}

	if err = push.SaveConfig(ctx, &req.Config); err != nil {
		res.SetError(gerror.Newf("Failed to save push settings: %v", err))
		return res, nil
	}

	res.SetSuccess("Push settings saved")
	return
}
