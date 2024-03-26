package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
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

	store      *database.DeployTaskStore
	spaceStore *database.SpaceStore
	ib         imagebuilder.Builder
	ir         imagerunner.Runner

	nextLock *sync.Mutex
}

func NewFIFOScheduler(ib imagebuilder.Builder, ir imagerunner.Runner) Scheduler {
	s := &FIFOScheduler{}
	// TODO:allow config
	s.timeout = 30 * time.Minute
	s.store = database.NewDeployTaskStore()
	s.spaceStore = database.NewSpaceStore()

	// allow concurrent deployment tasks
	s.tasks = make(chan Runner, 5)
	// s.ib = imagebuilder.NewLocalBuilder()
	// s.ir = imagerunner.NewLocalRunner()
	s.ib = ib
	s.ir = ir
	s.nextLock = &sync.Mutex{}
	return s
}

// Run will load tasks and run them currently
func (rs *FIFOScheduler) Run() error {
	slog.Info("FIFOScheduler run started")

	go func() {
		for count := 0; count <= cap(rs.tasks); count++ {
			_, err := rs.next()
			if err != nil {
				slog.Error("failed to get next task", "error", err)
				continue
			}
		}
	}()

	slog.Debug("scheudler try to loop through tasks channel")
	for t := range rs.tasks {
		go func(t Runner) {
			slog.Debug("dequeue a task to run", slog.Any("task", t.WatchID()))
			ctx, cancel := context.WithTimeout(context.Background(), rs.timeout)
			defer cancel()

			if err := t.Run(ctx); err != nil {
				slog.Error("failed to run task", slog.Any("error", err), slog.Any("task", t.WatchID()))
				rs.failDeployFollowingTasks(t.WatchID(), err.Error())
			}

			rs.next()
		}(t)
	}

	return nil
}

func (rs *FIFOScheduler) Queue(deployTaskID int64) error {
	slog.Info("queue next task", slog.Int64("deploy_task_id", deployTaskID))
	// simply trigger next task
	rs.next()

	return nil
}

// run next task
func (rs *FIFOScheduler) next() (Runner, error) {
	rs.nextLock.Lock()
	slog.Debug("FIFOScheduler try to get next task", slog.Any("last", rs.last))
	defer rs.nextLock.Unlock()

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
		slog.Debug("GetNewTaskFirst", slog.Any("deploy_task", deployTask), slog.Any("error", err))
	} else {
		deployTask, err = rs.store.GetNewTaskAfter(ctx, rs.last.ID)
		slog.Debug("GetNewTaskAfter", slog.Any("deploy_task", deployTask), slog.Any("last", rs.last.ID), slog.Any("error", err))
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Debug("no more tasks to run, schedule a sleeping task")
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
	var s *database.Space
	s, err = rs.spaceStore.ByID(ctx, deployTask.Deploy.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("get space failed, %w", err)
	}
	// for build task
	if deployTask.TaskType == 0 {
		t = NewBuidRunner(rs.ib, s, deployTask)
	} else {
		t = NewDeployRunner(rs.ir, s, deployTask)
	}

	rs.last = deployTask
	rs.tasks <- t
	slog.Info("enqueue next task", slog.Any("task", t.WatchID()))
	return t, err
}

func (rs *FIFOScheduler) failDeployFollowingTasks(deploytaskID int64, reason string) {
	slog.Info("scheduler fail following tasks", slog.Any("deploy_task_id", deploytaskID))
	t, _ := rs.store.GetDeployTask(context.Background(), deploytaskID)

	dps, err := rs.store.GetDeployTasksOfDeploy(context.Background(), t.DeployID)
	if err != nil {
		slog.Error("failed to get tasks of deploy when check build status", slog.Any("error", err),
			slog.Int64("deploy_id", t.DeployID))
		return
	}

	// update following tasks to be failed to stop scheduler to run it
	for _, dp := range dps {
		// fail current task
		if dp.ID == t.ID {
			dp.Status = buildFailed
			dp.Message = reason
			continue
		}
		// tasks after current task
		if dp.ID > t.ID {
			dp.Status = cancelled
			dp.Message = "cancel as previous task failed"
		}
	}
	if err := rs.store.UpdateInTx(context.Background(), nil, []string{"status", "message"}, nil, dps...); err != nil {
		slog.Error("failed update deploy status to `BuildFailed`", slog.Int64("deploy_task_id", t.ID), "error", err)
		return
	}
}
