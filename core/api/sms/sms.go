package sms

import (
	"context"

	v1 "billionmail-core/api/sms/v1"
)

type ISmsV1 interface {
	GetConfig(ctx context.Context, req *v1.GetConfigReq) (res *v1.GetConfigRes, err error)
	SaveConfig(ctx context.Context, req *v1.SaveConfigReq) (res *v1.SaveConfigRes, err error)
	SendTest(ctx context.Context, req *v1.SendTestReq) (res *v1.SendTestRes, err error)
	SendGroup(ctx context.Context, req *v1.SendGroupReq) (res *v1.SendGroupRes, err error)
	ListLogs(ctx context.Context, req *v1.ListLogsReq) (res *v1.ListLogsRes, err error)
}
