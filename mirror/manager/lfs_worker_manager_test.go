package manager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/mirror"
)

func newTestManager(workerNum int) *Manager {
	if workerNum <= 0 {
		workerNum = 3
	}
	return &Manager{
		workerNumber:     workerNum,
		priorityTaskChan: make(chan database.MirrorTask),
		conChan:          make(chan int, workerNum),
		workers:          make(map[int]*Worker),
	}
}

type testWorker struct {
	ctx    context.Context
	runCh  chan struct{} // signals when Run is called and blocks until test releases
	doneCh chan struct{} // closed when Run should complete
}

func (w *testWorker) SetContext(ctx context.Context) {
	w.ctx = ctx
}

func (w *testWorker) Run(mt *database.MirrorTask) {
	w.runCh <- struct{}{}
	<-w.doneCh
}

func newTestWorker() *testWorker {
	return &testWorker{
		runCh:  make(chan struct{}, 1),
		doneCh: make(chan struct{}),
	}
}

func TestRunningTasks_HoldsLock(t *testing.T) {
	m := newTestManager(3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.workers[1] = &Worker{
		ID:          1,
		ctx:         ctx,
		cancel:      cancel,
		RunningTask: &database.MirrorTask{ID: 100, MirrorID: 10},
	}
	m.workers[2] = &Worker{
		ID:          2,
		ctx:         ctx,
		cancel:      cancel,
		RunningTask: &database.MirrorTask{ID: 200, MirrorID: 20},
	}

	// Concurrent reads should not panic
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tasks := m.RunningTasks()
			assert.NotNil(t, tasks)
		}()
	}

	// Concurrent writes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			m.mu.Lock()
			m.workers[3] = &Worker{
				ID:          3,
				ctx:         ctx,
				cancel:      cancel,
				RunningTask: &database.MirrorTask{ID: int64(j)},
			}
			m.mu.Unlock()
		}
	}()

	wg.Wait()

	tasks := m.RunningTasks()
	assert.Len(t, tasks, 3)
}

func TestStopWorker_RemovesWorkerFromMap(t *testing.T) {
	m := newTestManager(3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tw := newTestWorker()
	m.workers[1] = &Worker{
		ID:          1,
		ctx:         ctx,
		cancel:      cancel,
		Worker:      tw,
		RunningTask: &database.MirrorTask{ID: 100, MirrorID: 10},
	}

	// Verify worker exists before stop
	tasks := m.RunningTasks()
	assert.Len(t, tasks, 1)

	err := m.StopWorker(1)
	assert.NoError(t, err)

	// Verify worker is removed from map
	tasks = m.RunningTasks()
	assert.Len(t, tasks, 0)

	// Verify stopping non-existent worker returns error
	err = m.StopWorker(99)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker 99 not found")
}

func TestStopWorkerByTaskID_HandlesNilRunningTask(t *testing.T) {
	m := newTestManager(3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Worker with nil RunningTask should not cause panic
	m.workers[1] = &Worker{
		ID:          1,
		ctx:         ctx,
		cancel:      cancel,
		RunningTask: nil,
	}

	// This should not panic even with nil RunningTask
	found, err := m.StopWorkerByTaskID(10)
	assert.False(t, found)
	assert.Error(t, err)
	// Worker should still be in map (no match, no delete)
	assert.Len(t, m.workers, 1)
}

func TestStopWorkerByTaskID_FindsAndRemovesWorker(t *testing.T) {
	m := newTestManager(3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tw := newTestWorker()
	tw2 := newTestWorker()

	m.workers[1] = &Worker{
		ID:          1,
		ctx:         ctx,
		cancel:      cancel,
		Worker:      tw,
		RunningTask: &database.MirrorTask{ID: 100, MirrorID: 10},
	}
	m.workers[2] = &Worker{
		ID:          2,
		ctx:         ctx,
		cancel:      cancel,
		Worker:      tw2,
		RunningTask: &database.MirrorTask{ID: 200, MirrorID: 20},
	}

	// Stop worker for task 100
	found, err := m.StopWorkerByTaskID(100)
	assert.True(t, found)
	assert.NoError(t, err)

	// Worker 1 should be gone, worker 2 should remain
	tasks := m.RunningTasks()
	assert.Len(t, tasks, 1)
	_, exists := tasks[2]
	assert.True(t, exists)
}

func TestStopWorker_DoesNotDoubleSendToConChan(t *testing.T) {
	m := newTestManager(3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tw := newTestWorker()
	m.workers[1] = &Worker{
		ID:          1,
		ctx:         ctx,
		cancel:      cancel,
		Worker:      tw,
		RunningTask: &database.MirrorTask{ID: 100, MirrorID: 10},
	}

	// Drain conChan (it starts empty since we created manager directly)
	// Simulate startWorker's end-of-life send by starting a goroutine
	go func() {
		// Simulate the cleanup at end of startWorker: m.conChan <- id
		time.Sleep(50 * time.Millisecond)
		m.conChan <- 1
	}()

	// Call StopWorker - it should NOT send to conChan
	err := m.StopWorker(1)
	assert.NoError(t, err)

	// Wait for goroutine to send
	select {
	case id := <-m.conChan:
		assert.Equal(t, 1, id) // Exactly one ID received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for conChan")
	}

	// Verify no second send
	select {
	case id := <-m.conChan:
		t.Fatalf("unexpected second send to conChan: %d", id)
	case <-time.After(100 * time.Millisecond):
		// Expected - no second send
	}
}

func TestRunningTasks_EmptyMap(t *testing.T) {
	m := newTestManager(3)
	tasks := m.RunningTasks()
	assert.Empty(t, tasks)
}

func TestConChan_CapacityPreserved(t *testing.T) {
	m := newTestManager(5)

	// Fill conChan with initial IDs
	for i := 1; i <= 5; i++ {
		m.conChan <- i
	}

	// Consume and verify all IDs are present
	ids := make(map[int]bool)
	for i := 0; i < 5; i++ {
		select {
		case id := <-m.conChan:
			ids[id] = true
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for conChan")
		}
	}

	assert.Len(t, ids, 5)
	for i := 1; i <= 5; i++ {
		assert.True(t, ids[i], "missing id %d", i)
	}
}

func TestManager_RunningTasks_ConcurrentStopWorker(t *testing.T) {
	m := newTestManager(5)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Add multiple workers
	for i := 1; i <= 5; i++ {
		tw := newTestWorker()
		m.workers[i] = &Worker{
			ID:          i,
			ctx:         ctx,
			cancel:      cancel,
			Worker:      tw,
			RunningTask: &database.MirrorTask{ID: int64(i * 100), MirrorID: int64(i * 10)},
		}
	}

	var wg sync.WaitGroup
	// Concurrent stops and reads
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = m.StopWorker(idx%5 + 1)
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.RunningTasks()
		}()
	}

	wg.Wait()
	// If we reach here without panic, the lock is correctly preventing concurrent map access
}

// Ensure Worker implements mirror.LFSSyncWorker
var _ mirror.LFSSyncWorker = (*testWorker)(nil)

// Ensure testWorker satisfies the interface
func TestTestWorkerSatisfiesInterface(t *testing.T) {
	var w mirror.LFSSyncWorker = newTestWorker()
	require.NotNil(t, w)
}
