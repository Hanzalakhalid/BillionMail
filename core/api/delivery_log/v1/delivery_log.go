package v1

import (
	"billionmail-core/utility/types/api_v1"

	"github.com/gogf/gf/v2/frame/g"
)

type ListReq struct {
	g.Meta        `path:"/delivery_log/list" method:"get" tags:"DeliveryLog" summary:"List delivery results of every sent email"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Status        string `json:"status" dc:"Filter by status (sent, bounced, deferred, expired)" in:"query"`
	Keyword       string `json:"keyword" dc:"Search recipient or sender" in:"query"`
	StartTime     int    `json:"start_time" dc:"Unix seconds, inclusive" in:"query"`
	EndTime       int    `json:"end_time" dc:"Unix seconds, inclusive" in:"query"`
	Page          int    `json:"page" v:"min:1" dc:"Page number" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" dc:"Page size" in:"query" default:"20"`
}

type DeliveryLog struct {
	PostfixMessageId string  `json:"postfix_message_id"`
	LogTime          int     `json:"log_time"`
	Status           string  `json:"status"`
	Sender           string  `json:"sender"`
	Recipient        string  `json:"recipient"`
	MailProvider     string  `json:"mail_provider"`
	Dsn              string  `json:"dsn"`
	Relay            string  `json:"relay"`
	Delay            float64 `json:"delay"`
	Description      string  `json:"description" dc:"Raw response from the receiving server"`
	Reason           string  `json:"reason" dc:"Plain-language meaning of the status code"`
}

type ListRes struct {
	api_v1.StandardRes
	Data struct {
		Total int            `json:"total"`
		List  []*DeliveryLog `json:"list"`
	} `json:"data"`
}
