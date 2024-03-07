package deploy

import (
	"log/slog"
	"sync"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
)

type Monitor struct {
	store        *database.DeployTaskStore
	runningTasks []*monitorTask
	completed    chan *monitorTask
	interval     time.Duration

	reloadLock sync.Locker
	needReload bool
}

func NewMonitor() *Monitor {
	return &Monitor{
		completed:  make(chan *monitorTask, 10),
		interval:   10 * time.Second,
		reloadLock: &sync.Mutex{},
	}
}

func (m *Monitor) Run() {
	for {
		var tmpTasks []*monitorTask
		if m.needReload {
			// TODO:load monitor tasks from db
			//

			m.reloadLock.Lock()
			m.needReload = false
			m.runningTasks = tmpTasks
			m.reloadLock.Unlock()
		}

		m.checkStatus(m.runningTasks)
		time.Sleep(m.interval)
	}
}

// Complete notify when a task completed
func (m *Monitor) Complete() <-chan *monitorTask {
	return m.completed
}

func (m *Monitor) Watch(t Task) error {
	// TODO:create a monitor task
	//
	m.reloadLock.Lock()
	m.needReload = true
	m.reloadLock.Unlock()
	return nil
}

func (m *Monitor) checkStatus(runningTasks []*monitorTask) {
	slog.Info("start checking running tasks' status")
	for _, t := range runningTasks {
		// TODO:run in parallel
		switch t.data.TaskType {
		case 0:
			m.checkBuildStatus(t)
		case 1:
			m.checkDeployStatus(t)
		default: // unknown
			slog.Error("")
		}
	}
}

func (m *Monitor) checkBuildStatus(t Task) {
}

func (m *Monitor) checkDeployStatus(t Task) {
}
