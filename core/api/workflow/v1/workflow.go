package v1

import (
	"billionmail-core/internal/service/workflow"
	"billionmail-core/utility/types/api_v1"

	"github.com/gogf/gf/v2/frame/g"
)

type ListReq struct {
	g.Meta        `path:"/workflow/list" method:"get" tags:"Workflow" summary:"List automation workflows"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
}

type ListRes struct {
	api_v1.StandardRes
	Data []workflow.ListItem `json:"data"`
}

type DetailReq struct {
	g.Meta        `path:"/workflow/detail" method:"get" tags:"Workflow" summary:"Get one workflow"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Id            int    `json:"id" v:"required|min:1" in:"query" dc:"Workflow ID"`
}

type DetailRes struct {
	api_v1.StandardRes
	Data *workflow.Workflow `json:"data"`
}

type CreateReq struct {
	g.Meta        `path:"/workflow/create" method:"post" tags:"Workflow" summary:"Create a workflow (saved as draft)"`
	Authorization string            `json:"authorization" dc:"Authorization" in:"header"`
	Name          string            `json:"name" v:"required|max-length:200" dc:"Workflow name"`
	Trigger       workflow.Trigger  `json:"trigger" dc:"What starts the workflow"`
	Steps         []workflow.Step   `json:"steps" dc:"Ordered steps"`
	Settings      workflow.Settings `json:"settings" dc:"Quiet hours, smart sending, exit conditions"`
}

type CreateRes struct {
	api_v1.StandardRes
	Data struct {
		Id int `json:"id"`
	} `json:"data"`
}

type UpdateReq struct {
	g.Meta        `path:"/workflow/update" method:"post" tags:"Workflow" summary:"Update a workflow"`
	Authorization string            `json:"authorization" dc:"Authorization" in:"header"`
	Id            int               `json:"id" v:"required|min:1" dc:"Workflow ID"`
	Name          string            `json:"name" v:"required|max-length:200" dc:"Workflow name"`
	Trigger       workflow.Trigger  `json:"trigger" dc:"What starts the workflow"`
	Steps         []workflow.Step   `json:"steps" dc:"Ordered steps"`
	Settings      workflow.Settings `json:"settings" dc:"Quiet hours, smart sending, exit conditions"`
}

type UpdateRes struct {
	api_v1.StandardRes
}

type DeleteReq struct {
	g.Meta        `path:"/workflow/delete" method:"post" tags:"Workflow" summary:"Delete a workflow"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Id            int    `json:"id" v:"required|min:1" dc:"Workflow ID"`
}

type DeleteRes struct {
	api_v1.StandardRes
}

type SetStatusReq struct {
	g.Meta        `path:"/workflow/set_status" method:"post" tags:"Workflow" summary:"Turn a workflow on or off"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	Id            int    `json:"id" v:"required|min:1" dc:"Workflow ID"`
	Status        string `json:"status" v:"required|in:active,paused" dc:"active or paused"`
}

type SetStatusRes struct {
	api_v1.StandardRes
}

type RunsReq struct {
	g.Meta        `path:"/workflow/runs" method:"get" tags:"Workflow" summary:"Contacts inside a workflow"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	WorkflowId    int    `json:"workflow_id" v:"required|min:1" in:"query" dc:"Workflow ID"`
	Status        string `json:"status" in:"query" dc:"active, completed or exited"`
	Keyword       string `json:"keyword" in:"query" dc:"Search email"`
	Page          int    `json:"page" v:"min:1" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" in:"query" default:"20"`
}

type RunsRes struct {
	api_v1.StandardRes
	Data struct {
		Total int            `json:"total"`
		List  []workflow.Run `json:"list"`
	} `json:"data"`
}

type LogsReq struct {
	g.Meta        `path:"/workflow/logs" method:"get" tags:"Workflow" summary:"What a workflow did, step by step"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	WorkflowId    int    `json:"workflow_id" v:"required|min:1" in:"query" dc:"Workflow ID"`
	Status        string `json:"status" in:"query" dc:"done, skipped, failed or exited"`
	Page          int    `json:"page" v:"min:1" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" in:"query" default:"20"`
}

type LogsRes struct {
	api_v1.StandardRes
	Data struct {
		Total int                 `json:"total"`
		List  []workflow.LogEntry `json:"list"`
	} `json:"data"`
}

type StepStatsReq struct {
	g.Meta        `path:"/workflow/step_stats" method:"get" tags:"Workflow" summary:"Per-step report: sent, skipped, opened, clicked"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	WorkflowId    int    `json:"workflow_id" v:"required|min:1" in:"query" dc:"Workflow ID"`
}

type StepStatsRes struct {
	api_v1.StandardRes
	Data map[string]*workflow.StepStats `json:"data" dc:"Keyed by step path, e.g. 0, 2/y/0"`
}

type ReviewsReq struct {
	g.Meta        `path:"/workflow/reviews" method:"get" tags:"Workflow" summary:"Manual messages waiting for review"`
	Authorization string `json:"authorization" dc:"Authorization" in:"header"`
	WorkflowId    int    `json:"workflow_id" v:"required|min:1" in:"query" dc:"Workflow ID"`
	Status        string `json:"status" in:"query" dc:"pending, sent, failed or discarded"`
	Page          int    `json:"page" v:"min:1" in:"query" default:"1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100" in:"query" default:"20"`
}

type ReviewsRes struct {
	api_v1.StandardRes
	Data struct {
		Total int               `json:"total"`
		List  []workflow.Review `json:"list"`
	} `json:"data"`
}

type ResolveReviewsReq struct {
	g.Meta        `path:"/workflow/resolve_reviews" method:"post" tags:"Workflow" summary:"Send or discard manual messages"`
	Authorization string  `json:"authorization" dc:"Authorization" in:"header"`
	WorkflowId    int     `json:"workflow_id" v:"required|min:1" dc:"Workflow ID"`
	Ids           []int64 `json:"ids" v:"required" dc:"Review IDs"`
	Action        string  `json:"action" v:"required|in:send,discard" dc:"send or discard"`
}

type ResolveReviewsRes struct {
	api_v1.StandardRes
	Data struct {
		Done   int `json:"done"`
		Failed int `json:"failed"`
	} `json:"data"`
}
