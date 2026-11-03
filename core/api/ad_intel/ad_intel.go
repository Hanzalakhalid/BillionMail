package ad_intel

import (
	"context"

	v1 "billionmail-core/api/ad_intel/v1"
)

type IAdIntelV1 interface {
	GetConfig(ctx context.Context, req *v1.GetConfigReq) (res *v1.GetConfigRes, err error)
	SaveConfig(ctx context.Context, req *v1.SaveConfigReq) (res *v1.SaveConfigRes, err error)
	Search(ctx context.Context, req *v1.SearchReq) (res *v1.SearchRes, err error)
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	AddLead(ctx context.Context, req *v1.AddLeadReq) (res *v1.AddLeadRes, err error)
}
