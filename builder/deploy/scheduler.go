package deploy

import (
	"context"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type Deployer interface {
	Deploy(s types.Space) (deployID int64, err error)
}

var _ Deployer = (*deployer)(nil)

type deployer struct {
	s Scheduler

	store *database.DeployTaskStore
}

func NewDeployer() (Deployer, error) {
	s := &FIFOScheduler{}
	s.store = &database.DeployTaskStore{}
	return &deployer{s: s}, nil
}

func (d *deployer) Deploy(s types.Space) (int64, error) {
	deploy := &database.Deploy{
		GitPath: s.Path,
		// Env: s.Env,
		// Secret: s.Secret,
	}
	ctx := context.Background()
	// TODO:save deploy tasks in sql tx
	err := d.store.CreateDeploy(ctx, deploy)
	if err != nil {
		slog.Error("failed to create deploy in db", slog.Any("error", err))
		return -1, err
	}
	buildTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 0,
	}
	d.store.CreateDeployTask(ctx, buildTask)
	runTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 1,
	}
	d.store.CreateDeployTask(ctx, runTask)
	return deploy.ID, nil
}

type Scheduler interface {
	Run() error
}

// a Scheduler will run tasks in their arrival order
type FIFOScheduler struct {
	timeout time.Duration
	// number of parallel tasks
	parallel int
	last     *database.DeployTask

	monitor *Monitor
	store   *database.DeployTaskStore
}

func NewFIFOScheduler() Scheduler {
	s := &FIFOScheduler{}
	// TODO:allow config
	s.timeout = 30 * time.Minute
	s.parallel = 5
	s.monitor = NewMonitor()
	s.store = database.NewDeployTaskStore()
	go s.Run()

	return s
}

// Run will load tasks and run them currently
func (rs *FIFOScheduler) Run() error {
	for t := range rs.tasks() {
		go func(t Task) {
			ctx, cancel := context.WithTimeout(context.Background(), rs.timeout)
			defer cancel()

			if err := t.Run(ctx); err != nil {
				slog.Error("failed to run task", slog.Any("task", t))
				return
			}

			if err := rs.monitor.Watch(t); err != nil {
				slog.Error("failed to monitor task", slog.Any("error", err))
			}
		}(t)
	}

	return nil
}

func (rs *FIFOScheduler) tasks() <-chan Task {
	// allow concurrent deployment tasks
	tasks := make(chan Task, rs.parallel)
	for {
		t, err := rs.next()
		if err != nil {
			slog.Error("failed to fetch next deply task", slog.Any("error", err))
			time.Sleep(10 * time.Second)
		} else {
			// will block until one old task complete
			tasks <- t
		}
	}
}

// run next task
func (rs *FIFOScheduler) next() (Task, error) {
	var (
		deployTask *database.DeployTask
		t          Task
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
		return nil, err
	}
	rs.last = deployTask
	// for build task
	if deployTask.TaskType == 0 {
		t = &buildTask{data: deployTask}
	} else {
		t = &runTask{data: deployTask}
	}

	return t, err
}

type Task interface {
	Run(context.Context) error
}

var (
	_ Task = (*buildTask)(nil)
	_ Task = (*runTask)(nil)
)

// buildTask defines a docker image building task
type buildTask struct {
	// DeployID int
	data *database.DeployTask
}

// Run call image builder service to build a docker image
func (t *buildTask) Run(ctx context.Context) error { return nil }

// runTask defines a k8s image running task
type runTask struct {
	// DeployID int
	data *database.DeployTask
}

// Run call k8s image runner service to run a docker image
func (t *runTask) Run(ctx context.Context) error { return nil }

type monitorTask struct {
	data *database.DeployTask
}
