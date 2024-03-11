package monitor

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
)

type Monitor interface {
	Run()
	Watch(id int64) error
	Watching() int
	Complete() <-chan int64

	Status(ctx context.Context, deployID int64) (string, error)
	Logs(ctx context.Context, deployID int64) (string, error)
}

var _ Monitor = (*DeployMonitor)(nil)

type DeployMonitor struct {
	store        *database.DeployTaskStore
	runningTasks []*database.MonitorTask
	completed    chan int64
	interval     time.Duration

	reloadLock *sync.RWMutex
	needReload bool
}

func NewMonitor() Monitor {
	return &DeployMonitor{
		completed:  make(chan int64, 10),
		interval:   10 * time.Second,
		reloadLock: &sync.RWMutex{},
	}
}

func (m *DeployMonitor) Run() {
	// start up, load all tasks at first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	m.runningTasks, err = m.store.GetAllMonitorTasks(ctx)
	if err != nil {
		m.needReload = true
	}

	// schedule reloading
	for {
		time.Sleep(m.interval)

		if m.needReload {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			tmpTasks, err := m.store.GetAllMonitorTasks(ctx)
			cancel()

			if err != nil {
				slog.Error("failed to reload monitor tasks", "error", err)
			} else {
				m.reloadLock.Lock()
				m.needReload = false
				m.runningTasks = tmpTasks
				tmpTasks = nil
				m.reloadLock.Unlock()

				slog.Info("monitor tasks reloaded")
			}
		}

		m.reloadLock.RLock()
		tasksCopy := m.runningTasks
		m.reloadLock.RUnlock()
		m.checkStatus(tasksCopy)
	}
}

// Complete notify when a task completed
func (m *DeployMonitor) Complete() <-chan int64 {
	return m.completed
}

func (m *DeployMonitor) Watching() int {
	m.reloadLock.RLock()
	defer m.reloadLock.RUnlock()

	return len(m.runningTasks)
}

func (m *DeployMonitor) Watch(deployTaskID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	t := &database.MonitorTask{
		DeployTaskID: deployTaskID,
	}
	if err := m.store.CreateMonitorTask(ctx, t); err != nil {
		slog.Error("monitor task creation failed", "error", err)
		return err
	}

	m.reloadLock.Lock()
	m.needReload = true
	m.runningTasks = append(m.runningTasks, t)
	m.reloadLock.Unlock()
	return nil
}

func (m *DeployMonitor) Status(ctx context.Context, deployID int64) (string, error) {
	return "", nil
}

func (m *DeployMonitor) Logs(ctx context.Context, deployID int64) (string, error) {
	return "", nil
}

func (m *DeployMonitor) checkStatus(runningTasks []*database.MonitorTask) {
	slog.Info("start checking running tasks' status")
	for _, t := range runningTasks {
		// TODO:run in parallel
		switch t.TaskType {
		case 0:
			m.checkBuildStatus(t)
		case 1:
			m.checkRunStatus(t)
		default: // unknown
			slog.Error("")
		}
	}
}

func (m *DeployMonitor) checkBuildStatus(t *database.MonitorTask) {
	var status int
	// TODO:call buidling service

	switch status {
	case BuildPending:
		t.DeployTask.Status = status
		// change to buidling status
		t.Deploy.Status = DeployBuildPending
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.UpdateInTx(ctx, t.Deploy, t.DeployTask); err != nil {
			slog.Error("failed to change deploy status to `building`", "error", err)
		}
	case BuildInProgress:
		// if t.Status == status {
		// 	return
		// }

		t.Status = status
		// change to buidling status
		t.Deploy.Status = DeployBuildInProgress
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.UpdateInTx(ctx, t.Deploy, t.DeployTask); err != nil {
			slog.Error("failed to change deploy status to `building`", "error", err)
		}
	case BuildSucceed:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		t.Status = status
		// change to startup status
		t.Deploy.Status = DeployPrepareToRun
		if err := m.store.UpdateInTx(ctx, t.Deploy, t.DeployTask); err != nil {
			// notify for anyone cares
			m.completed <- t.DeployID

			m.store.DeleteMonitorTask(ctx, t.DeployTaskID)

			// remove from monitoring queue
			m.reloadLock.Lock()
			m.runningTasks = slices.DeleteFunc(m.runningTasks, func(element *database.MonitorTask) bool {
				return t.DeployTask.ID == element.DeployTask.ID
			})
			m.reloadLock.Unlock()

		}

	case BuildFailed:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		dps, err := m.store.GetDeployTasksOfDeploy(ctx, t.DeployID)
		if err != nil {
			slog.Error("failed to get tasks of deploy when check build status", slog.Any("error", err),
				slog.Int("deploy_id", int(t.DeployID)))
			return
		}

		// change to build failed status
		t.Deploy.Status = DeployBuildFailed
		// update following tasks to be failed to stop scheduler to run it
		for _, dp := range dps {
			// tasks from current task
			if dp.ID >= t.ID {
				dp.Status = BuildFailed
			}
		}
		if err := m.store.UpdateInTx(ctx, t.Deploy, dps...); err != nil {
			// notify for anyone cares
			m.completed <- t.DeployID

			m.store.DeleteMonitorTask(ctx, t.DeployTaskID)

			// remove from monitoring queue
			m.reloadLock.Lock()
			m.runningTasks = slices.DeleteFunc(m.runningTasks, func(element *database.MonitorTask) bool {
				return t.DeployTask.ID == element.DeployTask.ID
			})
			m.reloadLock.Unlock()

		}

	}
}

func (m *DeployMonitor) checkRunStatus(t *database.MonitorTask) {
	var status int
	// TODO:call buidling service

	switch status {
	case PrepareToRun:
	case StartUp:
		t.Status = status
		// change to buidling status
		t.Deploy.Status = DeployStartUp
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.UpdateInTx(ctx, t.Deploy, t.DeployTask); err != nil {
			slog.Error("failed to change run status to `StartUp`", "error", err)
		}
	case Running:
		// if t.Status == status {
		// 	return
		// }

		t.Status = status
		// change to buidling status
		t.Deploy.Status = DeployRunning
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.UpdateInTx(ctx, t.Deploy, t.DeployTask); err != nil {
			slog.Error("failed to change deploy status to `building`", "error", err)
		}
	case RunTimeError:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		t.Status = status
		// change to build failed status
		t.Deploy.Status = DeployRunTimeError
		if err := m.store.UpdateInTx(ctx, t.Deploy, t.DeployTask); err != nil {
			// notify for anyone cares
			m.completed <- t.DeployID

			m.store.DeleteMonitorTask(ctx, t.DeployTaskID)

			// remove from monitoring queue
			m.reloadLock.Lock()
			m.runningTasks = slices.DeleteFunc(m.runningTasks, func(element *database.MonitorTask) bool {
				return t.DeployTask.ID == element.DeployTask.ID
			})
			m.reloadLock.Unlock()

		}

	}
}
