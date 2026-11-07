package projects

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	api "billionmail-core/api/projects"
	v1 "billionmail-core/api/projects/v1"
	service "billionmail-core/internal/service/projects"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/net/ghttp"
)

type ControllerV1 struct{}

func NewV1() api.IProjectsV1 { return &ControllerV1{} }
func failure(ctx context.Context, err error) error {
	if request := ghttp.RequestFromCtx(ctx); request != nil {
		switch {
		case errors.Is(err, service.ErrConflict):
			request.Response.Status = http.StatusConflict
		case errors.Is(err, service.ErrUnavailable):
			request.Response.Status = http.StatusNotFound
		case errors.Is(err, tenancy.ErrPermissionDenied):
			request.Response.Status = http.StatusForbidden
		}
	}
	return err
}
func (*ControllerV1) List(ctx context.Context, req *v1.ListReq) (*v1.ListRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	result, err := service.List(ctx, tenant, req.Cursor, req.Limit)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return &result, nil
}
func (*ControllerV1) Get(ctx context.Context, req *v1.GetReq) (*v1.GetRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	result, err := service.Get(ctx, tenant, req.ID)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return &result, nil
}
func (*ControllerV1) Save(ctx context.Context, req *v1.SaveReq) (*v1.SaveRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	request := ghttp.RequestFromCtx(ctx)
	if request == nil {
		return nil, errors.New("HTTP request required")
	}
	result, err := service.Save(ctx, tenant, request.Header.Get("Idempotency-Key"), req.Input)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return &result, nil
}
func (*ControllerV1) Message(ctx context.Context, req *v1.MessageReq) (*v1.MessageRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	request := ghttp.RequestFromCtx(ctx)
	if request == nil {
		return nil, errors.New("HTTP request required")
	}
	result, err := service.AddMessage(ctx, tenant, request.Header.Get("Idempotency-Key"), req.ID, req.Content)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return &result, nil
}
func (*ControllerV1) Events(ctx context.Context, req *v1.EventsReq) (*v1.EventsRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	result, err := service.Events(ctx, tenant, req.ID, req.After, req.Limit)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return &result, nil
}
func (*ControllerV1) Stream(ctx context.Context, req *v1.StreamReq) (*v1.StreamRes, error) {
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	request := ghttp.RequestFromCtx(ctx)
	if request == nil {
		return nil, errors.New("HTTP request required")
	}
	after := req.After
	if value := request.Header.Get("Last-Event-ID"); value != "" {
		after, err = strconv.ParseInt(value, 10, 64)
		if err != nil || after < 0 {
			request.Response.Status = http.StatusBadRequest
			return nil, errors.New("invalid event cursor")
		}
	}
	page, err := service.Events(ctx, tenant, req.ID, after, 100)
	if err != nil {
		return nil, failure(ctx, err)
	}
	request.Response.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	request.Response.Header().Set("Cache-Control", "no-cache, no-transform")
	request.Response.Header().Set("X-Accel-Buffering", "no")
	request.Response.Write("retry: 2000\n\n")
	// Bound each connection; clients reconnect using the last durable event ID.
	deadline := time.NewTimer(25 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		for _, event := range page.Items {
			request.Response.Writef("id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, event.Payload)
		}
		request.Response.Flush()
		after = page.NextAfter
		if len(page.Items) < 100 {
			select {
			case <-ctx.Done():
				return nil, nil
			case <-deadline.C:
				return nil, nil
			case <-ticker.C:
			}
			request.Response.Write(": heartbeat\n\n")
		} else {
			select {
			case <-ctx.Done():
				return nil, nil
			case <-deadline.C:
				return nil, nil
			default:
			}
		}
		page, err = service.Events(ctx, tenant, req.ID, after, 100)
		if err != nil {
			return nil, nil
		}
	}
}
