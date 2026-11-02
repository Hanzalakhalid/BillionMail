package v1

import (
	"billionmail-core/internal/service/whatsapp"
	"billionmail-core/utility/types/api_v1"

	"github.com/gogf/gf/v2/frame/g"
)

type GetConfigReq struct {
	g.Meta        `path:"/whatsapp/config" method:"get" tags:"WhatsApp" summary:"Get WhatsApp Cloud API settings"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
}

type GetConfigRes struct {
	api_v1.StandardRes
	Data *whatsapp.Config `json:"data"`
}

type SaveConfigReq struct {
	g.Meta        `path:"/whatsapp/config" method:"post" tags:"WhatsApp" summary:"Save WhatsApp Cloud API settings"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	whatsapp.Config
}

type SaveConfigRes struct {
	api_v1.StandardRes
}

type ListTemplatesReq struct {
	g.Meta        `path:"/whatsapp/templates" method:"get" tags:"WhatsApp" summary:"List message templates of the WhatsApp Business Account"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
}

type ListTemplatesRes struct {
	api_v1.StandardRes
	Data []whatsapp.TemplateInfo `json:"data"`
}

// TemplateFields are the template inputs shared by the send requests.
type TemplateFields struct {
	Template string   `json:"template" v:"required" dc:"Approved template name"`
	Language string   `json:"language" v:"required" dc:"Template language code, e.g. en, en_US, ur"`
	Params   []string `json:"params" dc:"Values for {{1}}, {{2}}, ... in the template body"`
}

type SendTestReq struct {
	g.Meta        `path:"/whatsapp/send_test" method:"post" tags:"WhatsApp" summary:"Send a template to one phone number"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Phone         string `json:"phone" v:"required" dc:"Phone number"`
	TemplateFields
}

type SendTestRes struct {
	api_v1.StandardRes
}

type SendGroupReq struct {
	g.Meta        `path:"/whatsapp/send_group" method:"post" tags:"WhatsApp" summary:"Send a template to every contact in a group that has a phone"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	GroupId       int    `json:"group_id" v:"required|min:1" dc:"Contact group ID"`
	TemplateFields
}

type SendGroupRes struct {
	api_v1.StandardRes
	Data struct {
		Queued int `json:"queued" dc:"Number of contacts being messaged"`
	} `json:"data"`
}

type ListLogsReq struct {
	g.Meta        `path:"/whatsapp/logs" method:"get" tags:"WhatsApp" summary:"List sent WhatsApp messages"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Status        string `json:"status" dc:"Filter by status (sent, failed)" in:"query"`
	Keyword       string `json:"keyword" dc:"Search phone number" in:"query"`
	Page          int    `json:"page" v:"min:1" dc:"Page number" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" dc:"Page size" in:"query" default:"20"`
}

type WhatsappLog struct {
	Id         int64  `json:"id"`
	Phone      string `json:"phone"`
	Template   string `json:"template"`
	Params     string `json:"params"`
	MessageId  string `json:"message_id"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	GroupId    int    `json:"group_id"`
	CreateTime int    `json:"create_time"`
}

type ListLogsRes struct {
	api_v1.StandardRes
	Data struct {
		Total int            `json:"total"`
		List  []*WhatsappLog `json:"list"`
	} `json:"data"`
}
