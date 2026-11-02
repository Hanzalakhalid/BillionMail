package whatsapp

import (
	"context"

	v1 "billionmail-core/api/whatsapp/v1"
)

type IWhatsappV1 interface {
	GetConfig(ctx context.Context, req *v1.GetConfigReq) (res *v1.GetConfigRes, err error)
	SaveConfig(ctx context.Context, req *v1.SaveConfigReq) (res *v1.SaveConfigRes, err error)
	ListTemplates(ctx context.Context, req *v1.ListTemplatesReq) (res *v1.ListTemplatesRes, err error)
	SendTest(ctx context.Context, req *v1.SendTestReq) (res *v1.SendTestRes, err error)
	SendGroup(ctx context.Context, req *v1.SendGroupReq) (res *v1.SendGroupRes, err error)
	ListLogs(ctx context.Context, req *v1.ListLogsReq) (res *v1.ListLogsRes, err error)
}
