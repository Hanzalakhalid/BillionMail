package v1

import (
	"billionmail-core/internal/service/projects"
	"github.com/gogf/gf/v2/frame/g"
)

type ListReq struct {
	g.Meta `path:"/v1/projects/list" method:"get" tags:"Projects" summary:"List marketing projects"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit" d:"30" v:"between:1,100"`
}
type ListRes = projects.Page
type GetReq struct {
	g.Meta `path:"/v1/projects/get" method:"get" tags:"Projects" summary:"Recover a marketing project"`
	ID     string `json:"id" v:"required"`
}
type GetRes = projects.Project
type SaveReq struct {
	g.Meta `path:"/v1/projects/save" method:"post" tags:"Projects" summary:"Save project discovery requirements"`
	projects.Input
}
type SaveRes = projects.Project
type MessageReq struct {
	g.Meta  `path:"/v1/projects/messages" method:"post" tags:"Projects" summary:"Append a user message"`
	ID      string `json:"id" v:"required"`
	Content string `json:"content" v:"required"`
}
type MessageRes = projects.Message
type EventsReq struct {
	g.Meta `path:"/v1/projects/events" method:"get" tags:"Projects" summary:"Recover project events after a durable cursor"`
	ID     string `json:"id" v:"required"`
	After  int64  `json:"after" v:"min:0"`
	Limit  int    `json:"limit" d:"100" v:"between:1,100"`
}
type EventsRes = projects.EventsPage
type StreamReq struct {
	g.Meta `path:"/v1/projects/stream" method:"get" tags:"Projects" summary:"Stream reconnectable project events"`
	ID     string `json:"id" v:"required"`
	After  int64  `json:"after" v:"min:0"`
}
type StreamRes struct{}
