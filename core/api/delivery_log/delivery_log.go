package delivery_log

import (
	"context"

	v1 "billionmail-core/api/delivery_log/v1"
)

type IDeliveryLogV1 interface {
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
}
