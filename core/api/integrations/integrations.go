package integrations

import (
	v1 "billionmail-core/api/integrations/v1"
	"context"
)

type IIntegrationsV1 interface {
	List(context.Context, *v1.ListReq) (*v1.ListRes, error)
	Create(context.Context, *v1.CreateReq) (*v1.CreateRes, error)
	Rotate(context.Context, *v1.RotateReq) (*v1.RotateRes, error)
	Disconnect(context.Context, *v1.DisconnectReq) (*v1.DisconnectRes, error)
}
