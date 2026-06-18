package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror"
)

var (
	once    sync.Once
	manager *Manager
	err     error
)

var expectedMirrorTaskStatus = []types.MirrorTaskStatus{
	types.MirrorRepoSyncFinished,
}

func InitManger(cfg *config.Config) error {
	once.Do(func() {
		manager = &Manager{
			workerNumber:     cfg.Mirror.WorkerNumber,
			priorityTaskChan: make(chan database.MirrorTask),
			mirrorTaskStore:  database.NewMirrorTaskStore(),
			config:           cfg,
			conChan:          make(chan int, cfg.Mirror.WorkerNumber),
			workers:          make(map[int]*Worker),
		}
	})
	return err
}

type Manager struct {
	config           *config.Config
	priorityTaskChan chan database.MirrorTask
	workerNumber     int
	workers          map[int]*Worker
	mu               sync.Mutex
	mirrorTaskStore  database.MirrorTaskStore
	conChan          chan int
}

type Worker struct {
	ID          int
	ctx         context.Context
	cancel      context.CancelFunc
	Worker      mirror.LFSSyncWorker
	RunningTask *database.MirrorTask
}

func GetManager(cfg *config.Config) (*Manager, error) {
	if manager == nil {
		err := InitManger(cfg)
		if err != nil {
			return nil, err
		}
	}
	return manager, nil
}

func (m *Manager) StopWorker(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if worker, ok := m.workers[id]; ok {
		worker.cancel()
		delete(m.workers, id)
	} else {
		return fmt.Errorf("worker %d not found", id)
	}

	return nil
}

func (m *Manager) StopWorkerByTaskID(taskID int64) (bool, error) {
	var found bool
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, worker := range m.workers {
		if worker.RunningTask != nil && worker.RunningTask.ID == taskID {
			found = true
			worker.cancel()
			delete(m.workers, id)
		}
	}

	if !found {
		return false, fmt.Errorf("worker for mirror %d not found", taskID)
	}

	return true, nil
}

func (m *Manager) ReRun(id int, mt *database.MirrorTask) error {
	if id == 0 {
		id = 1
	}

	m.mu.Lock()
	if worker, ok := m.workers[id]; ok {
		worker.cancel()
		delete(m.workers, id)
	}
	m.mu.Unlock()

	m.priorityTaskChan <- *mt

	return nil
}

func (m *Manager) Start() {
	ctx := context.Background()
	resetCount, err := m.mirrorTaskStore.ResetRunningTasks(ctx, types.MirrorLfsSyncStart, types.MirrorRepoSyncFinished)
	if err != nil {
		slog.Error("failed to reset running tasks", slog.Any("error", err))
	} else if resetCount > 0 {
		slog.Info("reset running tasks to repo_synced status", slog.Int("count", resetCount))
	}

	for i := 1; i <= m.workerNumber; i++ {
		m.conChan <- i
	}

	for id := range m.conChan {
		select {
		case mt := <-m.priorityTaskChan:
			go m.startWorker(id, &mt)
		default:
			go m.claimAndStartWorker(id)
		}
	}
}

func (m *Manager) startWorker(id int, mt *database.MirrorTask) {
	lfsSyncWorker, err := mirror.NewLFSSyncWorker(m.config, id)
	if err != nil {
		slog.Error("failed to create lfs sync worker", slog.Any("error", err))
		m.conChan <- id
		return
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	lfsSyncWorker.SetContext(ctx)

	m.mu.Lock()

	currentTask, err := m.mirrorTaskStore.FindByID(context.Background(), mt.ID)
	if err == nil && currentTask.Status == types.MirrorCanceled {
		m.mu.Unlock()
		cancel()
		m.conChan <- id
		return
	}

	for workerID, worker := range m.workers {
		if worker.RunningTask.ID == mt.ID {
			slog.Warn("worker for mirror is running, cancel it", slog.Any("mirrorID", mt.MirrorID), slog.Any("workerID", workerID))
			worker.cancel()
			delete(m.workers, workerID)
		}
	}
	m.workers[id] = &Worker{
		ID:          id,
		ctx:         ctx,
		cancel:      cancel,
		Worker:      lfsSyncWorker,
		RunningTask: mt,
	}
	m.mu.Unlock()

	lfsSyncWorker.Run(mt)
	m.mu.Lock()
	if w, ok := m.workers[id]; ok && w.Worker == lfsSyncWorker {
		delete(m.workers, id)
	}
	m.mu.Unlock()
	m.conChan <- id
}

func (m *Manager) claimAndStartWorker(id int) {
	ctx := context.Background()
	task, err := m.mirrorTaskStore.GetHighestPriorityByTaskStatus(ctx, expectedMirrorTaskStatus)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to claim task from db", slog.Any("error", err))
		}
		time.Sleep(5 * time.Second)
		m.conChan <- id
		return
	}
	m.startWorker(id, &task)
}

func (m *Manager) RunningTasks() map[int]database.MirrorTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	tasks := make(map[int]database.MirrorTask)
	for id, worker := range m.workers {
		tasks[id] = *worker.RunningTask
	}
	return tasks
}
