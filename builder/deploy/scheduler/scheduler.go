package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/deploy/monitor"
	"opencsg.com/csghub-server/builder/store/database"
)

type Scheduler interface {
	Run() error
	Queue(deployTaskID int64) error
}

// a Scheduler will run tasks in their arrival order
type FIFOScheduler struct {
	timeout time.Duration
	// parallel running tasks
	tasks chan Runner
	last  *database.DeployTask

	monitor monitor.Monitor
	store   *database.DeployTaskStore
}

func NewFIFOScheduler() Scheduler {
	s := &FIFOScheduler{}
	// TODO:allow config
	s.timeout = 30 * time.Minute
	s.monitor = monitor.NewMonitor()
	s.store = database.NewDeployTaskStore()

	// allow concurrent deployment tasks
	s.tasks = make(chan Runner, 5)
	return s
}

// Run will load tasks and run them currently
func (rs *FIFOScheduler) Run() error {
	go func() {
		for id := range rs.monitor.Complete() {
			slog.Info("task completed", "id", id)
			// TODO:update delopy task status

			rs.next()
		}
	}()

	for count := rs.monitor.Watching(); count <= cap(rs.tasks); count++ {
		r, err := rs.next()
		if err != nil {
			slog.Error("failed to get next task", "error", err)
			continue
		}
		if err == nil && r == nil {
			// no more task
			break
		}
	}

	for t := range rs.tasks {
		go func(t Runner) {
			ctx, cancel := context.WithTimeout(context.Background(), rs.timeout)
			defer cancel()

			if err := t.Run(ctx); err != nil {
				slog.Error("failed to run task", slog.Any("task", t))
				return
			}

			if t.WatchID() > 0 {
				if err := rs.monitor.Watch(t.WatchID()); err != nil {
					slog.Error("failed to monitor task", slog.Any("error", err))
				}
			}
		}(t)
	}

	return nil
}

func (rs *FIFOScheduler) Queue(deployTaskID int64) error {
	// simply trigger next task
	rs.next()

	return nil
}

// run next task
func (rs *FIFOScheduler) next() (Runner, error) {
	var (
		deployTask *database.DeployTask
		t          Runner
		err        error
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if rs.last == nil {
		// TODO: save last task into db
		deployTask, err = rs.store.GetNewTaskFirst(ctx)
	} else {
		deployTask, err = rs.store.GetNewTaskAfter(ctx, rs.last.DeployID)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Info("no more tasks to run")
			// using a sleep task to pause the scheduler
			t = &sleepTask{
				du: 5 * time.Second,
			}
			rs.tasks <- t
			return t, nil
		} else {
			return nil, fmt.Errorf("db operation failed, %w", err)
		}
	}
	// for build task
	if deployTask.TaskType == 0 {
		t = &buildTask{data: deployTask}
	} else {
		t = &runTask{data: deployTask}
	}

	rs.last = deployTask
	rs.tasks <- t
	return t, err
}
