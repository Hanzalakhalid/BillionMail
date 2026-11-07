package integrations

import (
	"context"
	"fmt"

	api "billionmail-core/api/integrations"
	v1 "billionmail-core/api/integrations/v1"
	service "billionmail-core/internal/service/integrations"
	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/tenancy"
	"github.com/google/uuid"
)

type ControllerV1 struct{}

func NewV1() api.IIntegrationsV1 { return &ControllerV1{} }

func (*ControllerV1) List(ctx context.Context, _ *v1.ListReq) (*v1.ListRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.List(ctx, tenant)
	if err != nil {
		return nil, err
	}
	res := &v1.ListRes{Data: items}
	res.SetSuccess("Integrations retrieved")
	return res, nil
}
func (*ControllerV1) Create(ctx context.Context, req *v1.CreateReq) (*v1.CreateRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	store, err := secrets.NewPersistentStore(ctx)
	if err != nil {
		return nil, err
	}
	id, err := service.Create(ctx, tenant, store, req.Input)
	if err != nil {
		return nil, err
	}
	res := &v1.CreateRes{}
	res.Data.ID = id.String()
	res.SetSuccess("Integration saved; connection remains unverified")
	return res, nil
}
func (*ControllerV1) Rotate(ctx context.Context, req *v1.RotateReq) (*v1.RotateRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid integration ID")
	}
	store, err := secrets.NewPersistentStore(ctx)
	if err != nil {
		return nil, err
	}
	if err = service.Rotate(ctx, tenant, store, id, req.Secret); err != nil {
		return nil, err
	}
	res := &v1.RotateRes{}
	res.SetSuccess("Credential rotated; connection remains unverified")
	return res, nil
}
func (*ControllerV1) Disconnect(ctx context.Context, req *v1.DisconnectReq) (*v1.DisconnectRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid integration ID")
	}
	if err = service.Disconnect(ctx, tenant, id); err != nil {
		return nil, err
	}
	res := &v1.DisconnectRes{}
	res.SetSuccess("Integration disconnected")
	return res, nil
}
