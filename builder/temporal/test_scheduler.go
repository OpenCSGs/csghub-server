package temporal

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
)

type TestScheduler struct {
	workflowOptions map[string]client.ScheduleOptions
}

func NewTestScheduler() *TestScheduler {
	return &TestScheduler{
		workflowOptions: map[string]client.ScheduleOptions{},
	}
}

func (ts *TestScheduler) Create(ctx context.Context, options client.ScheduleOptions) (client.ScheduleHandle, error) {
	ts.workflowOptions[options.ID] = options
	return nil, nil
}

func (ts *TestScheduler) Execute(id string, env *testsuite.TestWorkflowEnvironment) {
	ops, ok := ts.workflowOptions[id]
	if !ok {
		panic(fmt.Sprintf("%s not found", id))
	}
	act := ops.Action.(*client.ScheduleWorkflowAction)
	env.ExecuteWorkflow(act.Workflow, act.Args...)
}
