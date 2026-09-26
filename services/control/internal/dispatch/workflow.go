package dispatch

import "go.temporal.io/sdk/workflow"

const TaskQueue = "gridos-dispatch"

type Input struct {
	EventID string
}

func Workflow(workflow.Context, Input) error {
	return nil
}
