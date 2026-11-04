package companies

import "billionmail-core/api/companies"

type ControllerV1 struct{}

func NewV1() companies.ICompaniesV1 { return &ControllerV1{} }
