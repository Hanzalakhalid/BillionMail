package workflow

import (
	"billionmail-core/api/workflow"
)

type ControllerV1 struct{}

func NewV1() workflow.IWorkflowV1 {
	return &ControllerV1{}
}
