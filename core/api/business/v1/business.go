package v1

import (
	"billionmail-core/internal/service/business"
	"github.com/gogf/gf/v2/frame/g"
)

type GetReq struct {
	g.Meta `path:"/v1/business/profile" method:"get" tags:"Business" summary:"Get workspace business profile"`
}
type GetRes struct {
	Profile *business.Profile `json:"profile"`
}
type SaveReq struct {
	g.Meta `path:"/v1/business/profile" method:"put" tags:"Business" summary:"Save workspace business profile"`
	business.Input
}
type SaveRes = business.Profile
