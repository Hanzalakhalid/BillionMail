package push

import (
	"context"

	v1 "billionmail-core/api/push/v1"
)

type IPushV1 interface {
	GetConfig(ctx context.Context, req *v1.GetConfigReq) (res *v1.GetConfigRes, err error)
	SaveConfig(ctx context.Context, req *v1.SaveConfigReq) (res *v1.SaveConfigRes, err error)
	Send(ctx context.Context, req *v1.SendReq) (res *v1.SendRes, err error)
	ListLogs(ctx context.Context, req *v1.ListLogsReq) (res *v1.ListLogsRes, err error)
}
