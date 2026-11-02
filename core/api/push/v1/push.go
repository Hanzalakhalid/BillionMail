package v1

import (
	"billionmail-core/internal/service/push"
	"billionmail-core/utility/types/api_v1"

	"github.com/gogf/gf/v2/frame/g"
)

type GetConfigReq struct {
	g.Meta        `path:"/push/config" method:"get" tags:"Push" summary:"Get push notification settings"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
}

type GetConfigRes struct {
	api_v1.StandardRes
	Data *push.Config `json:"data"`
}

type SaveConfigReq struct {
	g.Meta        `path:"/push/config" method:"post" tags:"Push" summary:"Save push notification settings"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	push.Config
}

type SaveConfigRes struct {
	api_v1.StandardRes
}

type SendReq struct {
	g.Meta        `path:"/push/send" method:"post" tags:"Push" summary:"Send a push notification to all subscribers or a contact group"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Audience      string `json:"audience" v:"required|in:all,group" dc:"all: every subscriber, group: contacts of a group"`
	GroupId       int    `json:"group_id" v:"required-if:audience,group" dc:"Contact group ID (audience=group)"`
	Title         string `json:"title" v:"max-length:100" dc:"Title"`
	Message       string `json:"message" v:"required|max-length:500" dc:"Message"`
	Url           string `json:"url" v:"url" dc:"Opened on click"`
	Image         string `json:"image" v:"url" dc:"Image URL"`
}

type SendRes struct {
	api_v1.StandardRes
	Data struct {
		Targeted int `json:"targeted" dc:"Contacts targeted (group sends only)"`
	} `json:"data"`
}

type ListLogsReq struct {
	g.Meta        `path:"/push/logs" method:"get" tags:"Push" summary:"List sent push notifications"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Status        string `json:"status" dc:"Filter by status (sent, failed)" in:"query"`
	Page          int    `json:"page" v:"min:1" dc:"Page number" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" dc:"Page size" in:"query" default:"20"`
}

type PushLog struct {
	Id             int64  `json:"id"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	Url            string `json:"url"`
	Audience       string `json:"audience"`
	Targeted       int    `json:"targeted"`
	NotificationId string `json:"notification_id"`
	Status         string `json:"status"`
	Error          string `json:"error"`
	CreateTime     int    `json:"create_time"`
}

type ListLogsRes struct {
	api_v1.StandardRes
	Data struct {
		Total int        `json:"total"`
		List  []*PushLog `json:"list"`
	} `json:"data"`
}
