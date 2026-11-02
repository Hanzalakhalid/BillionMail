package v1

import (
	"billionmail-core/internal/service/sms"
	"billionmail-core/utility/types/api_v1"

	"github.com/gogf/gf/v2/frame/g"
)

type GetConfigReq struct {
	g.Meta        `path:"/sms/config" method:"get" tags:"SMS" summary:"Get SMS provider settings"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
}

type GetConfigRes struct {
	api_v1.StandardRes
	Data *sms.Config `json:"data"`
}

type SaveConfigReq struct {
	g.Meta        `path:"/sms/config" method:"post" tags:"SMS" summary:"Save SMS provider settings"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	sms.Config
}

type SaveConfigRes struct {
	api_v1.StandardRes
}

type SendTestReq struct {
	g.Meta        `path:"/sms/send_test" method:"post" tags:"SMS" summary:"Send one SMS to a phone number"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Phone         string `json:"phone" v:"required" dc:"Phone number"`
	Message       string `json:"message" v:"required|max-length:1600" dc:"Message text"`
}

type SendTestRes struct {
	api_v1.StandardRes
}

type SendGroupReq struct {
	g.Meta        `path:"/sms/send_group" method:"post" tags:"SMS" summary:"Send SMS to every contact in a group that has a phone"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	GroupId       int    `json:"group_id" v:"required|min:1" dc:"Contact group ID"`
	Message       string `json:"message" v:"required|max-length:1600" dc:"Message text, supports {name}-style contact attributes"`
}

type SendGroupRes struct {
	api_v1.StandardRes
	Data struct {
		Queued int `json:"queued" dc:"Number of contacts being messaged"`
	} `json:"data"`
}

type ListLogsReq struct {
	g.Meta        `path:"/sms/logs" method:"get" tags:"SMS" summary:"List sent SMS"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Status        string `json:"status" dc:"Filter by status (sent, failed)" in:"query"`
	Keyword       string `json:"keyword" dc:"Search phone number" in:"query"`
	Page          int    `json:"page" v:"min:1" dc:"Page number" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" dc:"Page size" in:"query" default:"20"`
}

type SmsLog struct {
	Id         int64  `json:"id"`
	Phone      string `json:"phone"`
	Message    string `json:"message"`
	Provider   string `json:"provider"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	GroupId    int    `json:"group_id"`
	CreateTime int    `json:"create_time"`
}

type ListLogsRes struct {
	api_v1.StandardRes
	Data struct {
		Total int       `json:"total"`
		List  []*SmsLog `json:"list"`
	} `json:"data"`
}
