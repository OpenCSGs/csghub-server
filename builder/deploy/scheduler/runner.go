package scheduler

import (
	"context"
	"log/slog"
	"time"
)

type Runner interface {
	Run(context.Context) error
	// WatchID is the unique ID for monitor service to watch the running progress
	WatchID() int64
}

var (
	_ Runner = (*BuilderRunner)(nil)
	_ Runner = (*DeployRunner)(nil)
)

type sleepTask struct {
	du time.Duration
}

func (t *sleepTask) Run(ctx context.Context) error {
	slog.Debug("sleeping task running", slog.Duration("time", t.du))
	time.Sleep(t.du)
	return nil
}
func (t *sleepTask) WatchID() int64 { return 0 }

const cancelled = -1

const (
	buildPending    = 0
	buildInProgress = 1
	buildFailed     = 2
	buildSucceed    = 3
)

// sub deploy task status
const (
	deployPending      = 0
	deploying          = 1
	deployFailed       = 2
	deployStartUp      = 3
	deployRunning      = 4
	deployRunTimeError = 5
)
