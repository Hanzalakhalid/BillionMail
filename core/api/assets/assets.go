package assets

import (
	v1 "billionmail-core/api/assets/v1"
	"context"
)

type IAssetsV1 interface {
	List(context.Context, *v1.ListReq) (*v1.ListRes, error)
	BeginUpload(context.Context, *v1.BeginUploadReq) (*v1.BeginUploadRes, error)
	CompleteUpload(context.Context, *v1.CompleteUploadReq) (*v1.CompleteUploadRes, error)
	Download(context.Context, *v1.DownloadReq) (*v1.DownloadRes, error)
	Delete(context.Context, *v1.DeleteReq) (*v1.DeleteRes, error)
}
