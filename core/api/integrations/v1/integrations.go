package v1

import (
	"billionmail-core/internal/service/integrations"
	"billionmail-core/utility/types/api_v1"
	"github.com/gogf/gf/v2/frame/g"
)

type ListReq struct {
	g.Meta `path:"/v1/integrations/list" method:"get" tags:"Integrations" summary:"List workspace integrations"`
}
type ListRes struct {
	api_v1.StandardRes
	Data []integrations.Connection `json:"data"`
}

type CreateReq struct {
	g.Meta         `path:"/v1/integrations/create" method:"post" tags:"Integrations" summary:"Create encrypted integration"`
	IdempotencyKey string `json:"idempotency_key" in:"header"`
	integrations.Input
}
type CreateRes struct {
	api_v1.StandardRes
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

type RotateReq struct {
	g.Meta `path:"/v1/integrations/rotate" method:"post" tags:"Integrations" summary:"Rotate integration credential"`
	ID     string `json:"id" v:"required"`
	Secret string `json:"secret" v:"required"`
}
type RotateRes struct{ api_v1.StandardRes }

type DisconnectReq struct {
	g.Meta `path:"/v1/integrations/disconnect" method:"post" tags:"Integrations" summary:"Disconnect integration"`
	ID     string `json:"id" v:"required"`
}
type DisconnectRes struct{ api_v1.StandardRes }
