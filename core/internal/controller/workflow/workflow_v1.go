package workflow

import (
	"context"
	"fmt"

	v1 "billionmail-core/api/workflow/v1"
	"billionmail-core/internal/service/workflow"
)

func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error) {
	res = &v1.ListRes{}
	list, err := workflow.List(ctx)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data = list
	res.SetSuccess("Success")
	return res, nil
}

func (c *ControllerV1) Detail(ctx context.Context, req *v1.DetailReq) (res *v1.DetailRes, err error) {
	res = &v1.DetailRes{}
	w, err := workflow.Get(ctx, req.Id)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data = w
	res.SetSuccess("Success")
	return res, nil
}

func (c *ControllerV1) Create(ctx context.Context, req *v1.CreateReq) (res *v1.CreateRes, err error) {
	res = &v1.CreateRes{}
	id, err := workflow.Create(ctx, req.Name, req.Trigger, req.Steps, req.Settings)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data.Id = id
	res.SetSuccess("Workflow saved")
	return res, nil
}

func (c *ControllerV1) Update(ctx context.Context, req *v1.UpdateReq) (res *v1.UpdateRes, err error) {
	res = &v1.UpdateRes{}
	if err = workflow.Update(ctx, req.Id, req.Name, req.Trigger, req.Steps, req.Settings); err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Workflow saved")
	return res, nil
}

func (c *ControllerV1) Delete(ctx context.Context, req *v1.DeleteReq) (res *v1.DeleteRes, err error) {
	res = &v1.DeleteRes{}
	if err = workflow.Delete(ctx, req.Id); err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Workflow deleted")
	return res, nil
}

func (c *ControllerV1) SetStatus(ctx context.Context, req *v1.SetStatusReq) (res *v1.SetStatusRes, err error) {
	res = &v1.SetStatusRes{}
	if err = workflow.SetStatus(ctx, req.Id, req.Status); err != nil {
		res.SetError(err)
		return res, nil
	}
	if req.Status == workflow.StatusActive {
		res.SetSuccess("Workflow is on")
	} else {
		res.SetSuccess("Workflow paused")
	}
	return res, nil
}

func (c *ControllerV1) Runs(ctx context.Context, req *v1.RunsReq) (res *v1.RunsRes, err error) {
	res = &v1.RunsRes{}
	total, list, err := workflow.ListRuns(ctx, req.WorkflowId, req.Status, req.Keyword, req.Page, req.PageSize)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data.Total = total
	res.Data.List = list
	res.SetSuccess("Success")
	return res, nil
}

func (c *ControllerV1) Logs(ctx context.Context, req *v1.LogsReq) (res *v1.LogsRes, err error) {
	res = &v1.LogsRes{}
	total, list, err := workflow.ListLogs(ctx, req.WorkflowId, req.Status, req.Page, req.PageSize)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data.Total = total
	res.Data.List = list
	res.SetSuccess("Success")
	return res, nil
}

func (c *ControllerV1) StepStats(ctx context.Context, req *v1.StepStatsReq) (res *v1.StepStatsRes, err error) {
	res = &v1.StepStatsRes{}
	stats, err := workflow.GetStepStats(ctx, req.WorkflowId)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data = stats
	res.SetSuccess("Success")
	return res, nil
}

func (c *ControllerV1) Reviews(ctx context.Context, req *v1.ReviewsReq) (res *v1.ReviewsRes, err error) {
	res = &v1.ReviewsRes{}
	total, list, err := workflow.ListReviews(ctx, req.WorkflowId, req.Status, req.Page, req.PageSize)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data.Total = total
	res.Data.List = list
	res.SetSuccess("Success")
	return res, nil
}

func (c *ControllerV1) ResolveReviews(ctx context.Context, req *v1.ResolveReviewsReq) (res *v1.ResolveReviewsRes, err error) {
	res = &v1.ResolveReviewsRes{}
	send := req.Action == "send"
	done, failed, err := workflow.ResolveReviews(ctx, req.WorkflowId, req.Ids, send)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.Data.Done = done
	res.Data.Failed = failed
	switch {
	case !send:
		res.SetSuccess(fmt.Sprintf("%d messages discarded", done))
	case failed > 0:
		res.SetSuccess(fmt.Sprintf("%d sent, %d failed (see the review list for why)", done, failed))
	default:
		res.SetSuccess(fmt.Sprintf("%d messages sent", done))
	}
	return res, nil
}
