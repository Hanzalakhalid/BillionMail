package whatsapp

import (
	v1 "billionmail-core/api/whatsapp/v1"
	"billionmail-core/internal/service/whatsapp"
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
)

func toTemplate(f v1.TemplateFields) whatsapp.Template {
	return whatsapp.Template{Name: f.Template, Language: f.Language, Params: f.Params}
}

func (c *ControllerV1) SendTest(ctx context.Context, req *v1.SendTestReq) (res *v1.SendTestRes, err error) {
	res = &v1.SendTestRes{}

	if err = whatsapp.SendOne(ctx, req.Phone, toTemplate(req.TemplateFields)); err != nil {
		res.SetError(gerror.Newf("WhatsApp failed: %v", err))
		return res, nil
	}

	res.SetSuccess("WhatsApp message sent")
	return
}

func (c *ControllerV1) SendGroup(ctx context.Context, req *v1.SendGroupReq) (res *v1.SendGroupRes, err error) {
	res = &v1.SendGroupRes{}

	queued, err := whatsapp.SendToGroup(ctx, req.GroupId, toTemplate(req.TemplateFields))
	if err != nil {
		res.SetError(gerror.Newf("WhatsApp failed: %v", err))
		return res, nil
	}

	res.Data.Queued = queued
	res.SetSuccess("WhatsApp sending started")
	return
}
