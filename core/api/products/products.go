package products

import (
	v1 "billionmail-core/api/products/v1"
	"context"
)

type IProductsV1 interface {
	List(context.Context, *v1.ListReq) (*v1.ListRes, error)
	Get(context.Context, *v1.GetReq) (*v1.GetRes, error)
	Save(context.Context, *v1.SaveReq) (*v1.SaveRes, error)
	Versions(context.Context, *v1.VersionsReq) (*v1.VersionsRes, error)
	Delete(context.Context, *v1.DeleteReq) (*v1.DeleteRes, error)
}
