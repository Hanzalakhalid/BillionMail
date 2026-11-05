package chatbot

import (
	v1 "billionmail-core/api/chatbot/v1"
	"context"
)

type IChatbotV1 interface {
	List(context.Context, *v1.ListReq) (*v1.ListRes, error)
	Detail(context.Context, *v1.DetailReq) (*v1.DetailRes, error)
	Create(context.Context, *v1.CreateReq) (*v1.CreateRes, error)
	Update(context.Context, *v1.UpdateReq) (*v1.UpdateRes, error)
	Publish(context.Context, *v1.PublishReq) (*v1.PublishRes, error)
	Delete(context.Context, *v1.DeleteReq) (*v1.DeleteRes, error)
	Simulate(context.Context, *v1.SimulateReq) (*v1.SimulateRes, error)
}
