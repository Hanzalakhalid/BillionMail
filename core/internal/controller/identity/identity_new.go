package identity

import api "billionmail-core/api/identity"

type ControllerV1 struct{}

func NewV1() api.IIdentityV1 { return &ControllerV1{} }
