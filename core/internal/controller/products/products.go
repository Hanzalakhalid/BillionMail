package products

import (
	api "billionmail-core/api/products"
	v1 "billionmail-core/api/products/v1"
	service "billionmail-core/internal/service/products"
	"billionmail-core/internal/service/tenancy"
	"context"
	"errors"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
	"net/http"
)

type ControllerV1 struct{}

func NewV1() api.IProductsV1 { return &ControllerV1{} }
func identify(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, errors.New("valid product ID is required")
	}
	return id, nil
}
func failure(ctx context.Context, err error) error {
	if request := ghttp.RequestFromCtx(ctx); request != nil {
		switch {
		case errors.Is(err, service.ErrConflict):
			request.Response.Status = http.StatusConflict
		case errors.Is(err, service.ErrUnavailable):
			request.Response.Status = http.StatusNotFound
		case errors.Is(err, tenancy.ErrPermissionDenied):
			request.Response.Status = http.StatusForbidden
		}
	}
	return err
}
func (*ControllerV1) List(ctx context.Context, req *v1.ListReq) (*v1.ListRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	page, err := service.List(ctx, tenant, req.Cursor, req.Limit)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return &page, nil
}
func (*ControllerV1) Get(ctx context.Context, req *v1.GetReq) (*v1.GetRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := identify(req.ID)
	if err != nil {
		return nil, err
	}
	product, err := service.Get(ctx, tenant, id)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return &product, nil
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
	input, err := saveInput(req)
	if err != nil {
		request.Response.Status = http.StatusBadRequest
		return nil, err
	}
	product, err := service.Save(ctx, tenant, request.Header.Get("Idempotency-Key"), input)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return &product, nil
}

// Keep UUID arrays out of request binding: GoFrame's converter cannot safely
// bind their TextUnmarshaler implementation inside slices.
func saveInput(req *v1.SaveReq) (service.Input, error) {
	input := service.Input{Revision: req.Revision, Name: req.Name, SKU: req.SKU,
		Description: req.Description, Benefits: req.Benefits, URL: req.URL,
		Price: req.Price, Currency: req.Currency, AssetVersionIDs: []uuid.UUID{}}
	if req.ID != "" {
		id, err := uuid.Parse(req.ID)
		if err != nil {
			return input, errors.New("invalid product ID")
		}
		input.ID = id
	}
	for _, value := range req.AssetVersionIDs {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return input, errors.New("invalid asset version ID")
		}
		input.AssetVersionIDs = append(input.AssetVersionIDs, id)
	}
	return input, nil
}
func (*ControllerV1) Versions(ctx context.Context, req *v1.VersionsReq) (*v1.VersionsRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := identify(req.ID)
	if err != nil {
		return nil, err
	}
	if req.Before < 0 {
		return nil, errors.New("version cursor must be nonnegative")
	}
	items, err := service.Versions(ctx, tenant, id, req.Before)
	if err != nil {
		return nil, failure(ctx, err)
	}
	result := &v1.VersionsRes{Items: items}
	if len(items) == 30 {
		result.NextBefore = items[len(items)-1].Version
	}
	return result, nil
}
func (*ControllerV1) Delete(ctx context.Context, req *v1.DeleteReq) (*v1.DeleteRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := identify(req.ID)
	if err != nil {
		return nil, err
	}
	if err = service.Delete(ctx, tenant, id); err != nil {
		return nil, failure(ctx, err)
	}
	return &v1.DeleteRes{ID: id.String(), Status: "deleted"}, nil
}
