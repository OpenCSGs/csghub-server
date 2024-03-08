package scheduler

import (
	"context"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
)

type Runner interface {
	Run(context.Context) error
	// WatchID is the unique ID for monitor service to watch the running progress
	WatchID() int64
}

var (
	_ Runner = (*buildTask)(nil)
	_ Runner = (*runTask)(nil)
)

// buildTask defines a docker image building task
type buildTask struct {
	// DeployID int
	data *database.DeployTask
}

// Run call image builder service to build a docker image
func (t *buildTask) Run(ctx context.Context) error { return nil }
func (t *buildTask) WatchID() int64                { return t.data.DeployID }

// runTask defines a k8s image running task
type runTask struct {
	// DeployID int
	data *database.DeployTask
}

// Run call k8s image runner service to run a docker image
func (t *runTask) Run(ctx context.Context) error { return nil }
func (t *runTask) WatchID() int64                { return t.data.DeployID }

type sleepTask struct {
	du time.Duration
}

func (t *sleepTask) Run(ctx context.Context) error {
	time.Sleep(t.du)
	return nil
}
func (t *sleepTask) WatchID() int64 { return 0 }
