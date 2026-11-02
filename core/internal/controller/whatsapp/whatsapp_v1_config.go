package whatsapp

import (
	v1 "billionmail-core/api/whatsapp/v1"
	"billionmail-core/internal/service/whatsapp"
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) GetConfig(ctx context.Context, req *v1.GetConfigReq) (res *v1.GetConfigRes, err error) {
	res = &v1.GetConfigRes{}

	cfg, err := whatsapp.LoadConfig(ctx)
	if err != nil {
		g.Log().Error(ctx, "load whatsapp config: %v", err)
		res.SetError(gerror.New("Failed to load WhatsApp settings"))
		return res, nil
	}

	res.Data = cfg.Masked()
	res.SetSuccess("Success")
	return
}

func (c *ControllerV1) SaveConfig(ctx context.Context, req *v1.SaveConfigReq) (res *v1.SaveConfigRes, err error) {
	res = &v1.SaveConfigRes{}

	if err = whatsapp.SaveConfig(ctx, &req.Config); err != nil {
		res.SetError(gerror.Newf("Failed to save WhatsApp settings: %v", err))
		return res, nil
	}

	res.SetSuccess("WhatsApp settings saved")
	return
}

func (c *ControllerV1) ListTemplates(ctx context.Context, req *v1.ListTemplatesReq) (res *v1.ListTemplatesRes, err error) {
	res = &v1.ListTemplatesRes{}

	list, err := whatsapp.ListTemplates(ctx)
	if err != nil {
		res.SetError(gerror.Newf("Failed to load templates: %v", err))
		return res, nil
	}

	res.Data = list
	res.SetSuccess("Success")
	return
}
