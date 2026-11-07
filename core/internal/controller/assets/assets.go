package assets

import (
	"context"
	"errors"
	"net/http"

	api "billionmail-core/api/assets"
	v1 "billionmail-core/api/assets/v1"
	service "billionmail-core/internal/service/assets"
	"billionmail-core/internal/service/objectstorage"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
)

type ControllerV1 struct{}

func NewV1() api.IAssetsV1 { return &ControllerV1{} }

func (*ControllerV1) List(ctx context.Context, req *v1.ListReq) (*v1.ListRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.List(ctx, tenant, req.Cursor, req.Limit)
	if err != nil {
		return nil, err
	}
	return &items, nil
}

func (*ControllerV1) Delete(ctx context.Context, req *v1.DeleteReq) (*v1.DeleteRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, errors.New("invalid asset")
	}
	if err = service.Delete(ctx, tenant, id); err != nil {
		return nil, err
	}
	return &v1.DeleteRes{ID: id.String(), Status: "deleted"}, nil
}
func (*ControllerV1) BeginUpload(ctx context.Context, req *v1.BeginUploadReq) (*v1.BeginUploadRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	request := ghttp.RequestFromCtx(ctx)
	if request == nil {
		return nil, errors.New("HTTP request is required")
	}
	key := request.Header.Get("Idempotency-Key")
	store, err := objectstorage.New(ctx, objectstorage.ConfigFromEnv())
	if err != nil {
		return nil, err
	}
	upload, err := service.BeginUpload(ctx, tenant, store, key, req.UploadInput)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			request.Response.Status = http.StatusConflict
		}
		return nil, err
	}
	return &upload, nil
}
func (*ControllerV1) CompleteUpload(ctx context.Context, req *v1.CompleteUploadReq) (*v1.CompleteUploadRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.VersionID)
	if err != nil {
		return nil, errors.New("invalid asset version")
	}
	job, err := service.CompleteUpload(ctx, tenant, id)
	if err != nil {
		return nil, err
	}
	if request := ghttp.RequestFromCtx(ctx); request != nil {
		request.Response.Status = http.StatusAccepted
	}
	return &job, nil
}
func (*ControllerV1) Download(ctx context.Context, req *v1.DownloadReq) (*v1.DownloadRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.VersionID)
	if err != nil {
		return nil, errors.New("invalid asset version")
	}
	store, err := objectstorage.New(ctx, objectstorage.ConfigFromEnv())
	if err != nil {
		return nil, err
	}
	url, err := service.Download(ctx, tenant, store, id)
	if err != nil {
		return nil, err
	}
	return &v1.DownloadRes{URL: url}, nil
}
