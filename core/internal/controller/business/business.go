package business

import (
	api "billionmail-core/api/business"
	v1 "billionmail-core/api/business/v1"
	service "billionmail-core/internal/service/business"
	"billionmail-core/internal/service/tenancy"
	"context"
	"errors"
	"github.com/gogf/gf/v2/net/ghttp"
	"net/http"
)

type ControllerV1 struct{}

func NewV1() api.IBusinessV1 { return &ControllerV1{} }
func (*ControllerV1) Get(ctx context.Context, _ *v1.GetReq) (*v1.GetRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := service.Get(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return &v1.GetRes{Profile: profile}, nil
}
func (*ControllerV1) Save(ctx context.Context, req *v1.SaveReq) (*v1.SaveRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	request := ghttp.RequestFromCtx(ctx)
	if request == nil {
		return nil, errors.New("HTTP request required")
	}
	profile, err := service.Save(ctx, tenant, request.Header.Get("Idempotency-Key"), req.Input)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			request.Response.Status = http.StatusConflict
		}
		return nil, err
	}
	return &profile, nil
}
