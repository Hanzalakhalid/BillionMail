package workflow

import (
	"context"

	v1 "billionmail-core/api/workflow/v1"
)

type IWorkflowV1 interface {
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	Detail(ctx context.Context, req *v1.DetailReq) (res *v1.DetailRes, err error)
	Create(ctx context.Context, req *v1.CreateReq) (res *v1.CreateRes, err error)
	Update(ctx context.Context, req *v1.UpdateReq) (res *v1.UpdateRes, err error)
	Delete(ctx context.Context, req *v1.DeleteReq) (res *v1.DeleteRes, err error)
	SetStatus(ctx context.Context, req *v1.SetStatusReq) (res *v1.SetStatusRes, err error)
	Runs(ctx context.Context, req *v1.RunsReq) (res *v1.RunsRes, err error)
	Logs(ctx context.Context, req *v1.LogsReq) (res *v1.LogsRes, err error)
	StepStats(ctx context.Context, req *v1.StepStatsReq) (res *v1.StepStatsRes, err error)
	Reviews(ctx context.Context, req *v1.ReviewsReq) (res *v1.ReviewsRes, err error)
	ResolveReviews(ctx context.Context, req *v1.ResolveReviewsReq) (res *v1.ResolveReviewsRes, err error)
}
