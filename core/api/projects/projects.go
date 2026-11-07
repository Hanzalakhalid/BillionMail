package projects

import (
	v1 "billionmail-core/api/projects/v1"
	"context"
)

type IProjectsV1 interface {
	List(context.Context, *v1.ListReq) (*v1.ListRes, error)
	Get(context.Context, *v1.GetReq) (*v1.GetRes, error)
	Save(context.Context, *v1.SaveReq) (*v1.SaveRes, error)
	Message(context.Context, *v1.MessageReq) (*v1.MessageRes, error)
	Events(context.Context, *v1.EventsReq) (*v1.EventsRes, error)
	Stream(context.Context, *v1.StreamReq) (*v1.StreamRes, error)
}
