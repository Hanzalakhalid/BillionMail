package business

import (
	v1 "billionmail-core/api/business/v1"
	"context"
)

type IBusinessV1 interface {
	Get(context.Context, *v1.GetReq) (*v1.GetRes, error)
	Save(context.Context, *v1.SaveReq) (*v1.SaveRes, error)
}
