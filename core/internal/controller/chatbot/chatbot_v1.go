package chatbot

import (
	"context"

	v1 "billionmail-core/api/chatbot/v1"
	service "billionmail-core/internal/service/chatbot"
	"billionmail-core/internal/service/tenancy"
	"github.com/google/uuid"
)

func tenant(ctx context.Context) (tenancy.TenantContext, error) { return tenancy.Require(ctx) }
func parseID(value string) (uuid.UUID, error)                   { return uuid.Parse(value) }

func (c *ControllerV1) List(ctx context.Context, _ *v1.ListReq) (*v1.ListRes, error) {
	res := &v1.ListRes{}
	t, err := tenant(ctx)
	if err == nil {
		res.Data, err = service.List(ctx, t)
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Success")
	return res, nil
}
func (c *ControllerV1) Detail(ctx context.Context, req *v1.DetailReq) (*v1.DetailRes, error) {
	res := &v1.DetailRes{}
	t, err := tenant(ctx)
	var id uuid.UUID
	if err == nil {
		id, err = parseID(req.ID)
	}
	if err == nil {
		res.Data, err = service.Get(ctx, t, id)
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Success")
	return res, nil
}
func (c *ControllerV1) Create(ctx context.Context, req *v1.CreateReq) (*v1.CreateRes, error) {
	res := &v1.CreateRes{}
	t, err := tenant(ctx)
	var phoneID, id uuid.UUID
	if err == nil {
		phoneID, err = parseID(req.PhoneNumberID)
	}
	if err == nil {
		id, err = service.Create(ctx, t, req.Name, phoneID, req.Graph)
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data.ID = id.String()
	res.SetSuccess("Bot draft created")
	return res, nil
}
func (c *ControllerV1) Update(ctx context.Context, req *v1.UpdateReq) (*v1.UpdateRes, error) {
	res := &v1.UpdateRes{}
	t, err := tenant(ctx)
	var id, phoneID uuid.UUID
	if err == nil {
		id, err = parseID(req.ID)
	}
	if err == nil {
		phoneID, err = parseID(req.PhoneNumberID)
	}
	if err == nil {
		err = service.UpdateDraft(ctx, t, id, req.Name, phoneID, req.Graph)
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Bot draft saved")
	return res, nil
}
func (c *ControllerV1) Publish(ctx context.Context, req *v1.PublishReq) (*v1.PublishRes, error) {
	res := &v1.PublishRes{}
	t, err := tenant(ctx)
	var id uuid.UUID
	if err == nil {
		id, err = parseID(req.ID)
	}
	if err == nil {
		err = service.Publish(ctx, t, id)
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Bot published and active")
	return res, nil
}
func (c *ControllerV1) Delete(ctx context.Context, req *v1.DeleteReq) (*v1.DeleteRes, error) {
	res := &v1.DeleteRes{}
	t, err := tenant(ctx)
	var id uuid.UUID
	if err == nil {
		id, err = parseID(req.ID)
	}
	if err == nil {
		err = service.Delete(ctx, t, id)
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Bot deleted")
	return res, nil
}
func (c *ControllerV1) Simulate(ctx context.Context, req *v1.SimulateReq) (*v1.SimulateRes, error) {
	res := &v1.SimulateRes{}
	t, err := tenant(ctx)
	if err == nil {
		err = tenancy.Authorize(t, "flow.read")
	}
	if err == nil {
		res.Data, err = service.Execute(req.Graph, req.CurrentNode, req.Input)
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Success")
	return res, nil
}
