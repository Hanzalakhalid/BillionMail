package v1

import (
	"billionmail-core/internal/service/chatbot"
	"billionmail-core/utility/types/api_v1"
	"github.com/gogf/gf/v2/frame/g"
)

type ListReq struct {
	g.Meta `path:"/v1/chatbots" method:"get" tags:"WhatsApp Chatbots"`
}
type ListRes struct {
	api_v1.StandardRes
	Data []chatbot.Flow `json:"data"`
}

type DetailReq struct {
	g.Meta `path:"/v1/chatbots/{id}" method:"get" tags:"WhatsApp Chatbots"`
	ID     string `json:"id" in:"path" v:"required|uuid"`
}
type DetailRes struct {
	api_v1.StandardRes
	Data chatbot.Flow `json:"data"`
}

type CreateReq struct {
	g.Meta        `path:"/v1/chatbots" method:"post" tags:"WhatsApp Chatbots"`
	Name          string        `json:"name" v:"required|max-length:160"`
	PhoneNumberID string        `json:"phone_number_id" v:"required|uuid"`
	Graph         chatbot.Graph `json:"graph" v:"required"`
}
type CreateRes struct {
	api_v1.StandardRes
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

type UpdateReq struct {
	g.Meta        `path:"/v1/chatbots/{id}" method:"put" tags:"WhatsApp Chatbots"`
	ID            string        `json:"id" in:"path" v:"required|uuid"`
	Name          string        `json:"name" v:"required|max-length:160"`
	PhoneNumberID string        `json:"phone_number_id" v:"required|uuid"`
	Graph         chatbot.Graph `json:"graph" v:"required"`
}
type UpdateRes struct{ api_v1.StandardRes }

type PublishReq struct {
	g.Meta `path:"/v1/chatbots/{id}/publish" method:"post" tags:"WhatsApp Chatbots"`
	ID     string `json:"id" in:"path" v:"required|uuid"`
}
type PublishRes struct{ api_v1.StandardRes }

type DeleteReq struct {
	g.Meta `path:"/v1/chatbots/{id}" method:"delete" tags:"WhatsApp Chatbots"`
	ID     string `json:"id" in:"path" v:"required|uuid"`
}
type DeleteRes struct{ api_v1.StandardRes }

type SimulateReq struct {
	g.Meta      `path:"/v1/chatbots/simulate" method:"post" tags:"WhatsApp Chatbots"`
	Graph       chatbot.Graph `json:"graph" v:"required"`
	CurrentNode string        `json:"current_node"`
	Input       string        `json:"input"`
}
type SimulateRes struct {
	api_v1.StandardRes
	Data chatbot.EngineResult `json:"data"`
}
