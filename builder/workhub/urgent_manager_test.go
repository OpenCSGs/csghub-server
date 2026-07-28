package workhub

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
)

type fakeQueueController struct {
	mu                       sync.Mutex
	removeCalls              int
	addCalls                 int
	removeErr                error
	addErr                   error
	addFailures              int
	addFailureLeavesProducer bool
	producerAdded            bool
}

func (c *fakeQueueController) RemoveQueue(ctx context.Context, queue string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeCalls++
	if c.removeErr == nil {
		c.producerAdded = false
	}
	return c.removeErr
}

func (c *fakeQueueController) AddQueue(queue string, config river.QueueConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addCalls++
	if c.producerAdded {
		return &river.QueueAlreadyAddedError{Name: queue}
	}
	if c.addFailures > 0 {
		c.addFailures--
		c.producerAdded = c.addFailureLeavesProducer
		return c.addErr
	}
	c.producerAdded = true
	return nil
}

func (c *fakeQueueController) calls() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.removeCalls, c.addCalls
}

// controlledRemoveQueueController exposes the two RemoveQueue phases used when an abandoned cycle is reconciled.
type controlledRemoveQueueController struct {
	mu               sync.Mutex
	removeCalls      int
	addCalls         int
	firstStarted     chan struct{}
	firstCanceled    chan struct{}
	allowFirst       chan struct{}
	reconcileStarted chan struct{}
	allowReconcile   chan struct{}
	addCalled        chan struct{}
}

// newControlledRemoveQueueController creates queue synchronization gates for preemption tests.
func newControlledRemoveQueueController() *controlledRemoveQueueController {
	return &controlledRemoveQueueController{
		firstStarted:     make(chan struct{}),
		firstCanceled:    make(chan struct{}),
		allowFirst:       make(chan struct{}),
		reconcileStarted: make(chan struct{}),
		allowReconcile:   make(chan struct{}),
		addCalled:        make(chan struct{}, 1),
	}
}

// RemoveQueue blocks the first removal until cancellation and gates the reconciliation removal.
func (c *controlledRemoveQueueController) RemoveQueue(ctx context.Context, queue string) error {
	c.mu.Lock()
	c.removeCalls++
	call := c.removeCalls
	c.mu.Unlock()

	switch call {
	case 1:
		close(c.firstStarted)
		select {
		case <-ctx.Done():
			close(c.firstCanceled)
			return ctx.Err()
		case <-c.allowFirst:
			return nil
		}
	case 2:
		close(c.reconcileStarted)
		select {
		case <-c.allowReconcile:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	default:
		return nil
	}
}

// AddQueue records restoration of the normal queue.
func (c *controlledRemoveQueueController) AddQueue(queue string, config river.QueueConfig) error {
	c.mu.Lock()
	c.addCalls++
	c.mu.Unlock()
	select {
	case c.addCalled <- struct{}{}:
	default:
	}
	return nil
}

// calls returns the recorded RemoveQueue and AddQueue call counts.
func (c *controlledRemoveQueueController) calls() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.removeCalls, c.addCalls
}

// synchronizedLogBuffer safely captures logs written by asynchronous manager callbacks.
type synchronizedLogBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

// Write appends one log record while holding the buffer lock.
func (b *synchronizedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

// String returns the captured logs while holding the buffer lock.
func (b *synchronizedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func newTestUrgentManager(controller localQueueController, idleDelay time.Duration) *UrgentManager {
	return NewUrgentManager(UrgentManagerConfig{
		QueueController: controller,
		NormalQueue:     MirrorRepoQueue,
		NormalQueueConfig: river.QueueConfig{
			MaxWorkers: 2,
		},
		UrgentIdleDelay: idleDelay,
	})
}

// captureUrgentManagerLogs installs a synchronized JSON logger for one test.
func captureUrgentManagerLogs(t *testing.T) *synchronizedLogBuffer {
	t.Helper()
	var output synchronizedLogBuffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })
	return &output
}

// TestUrgentManagerLogsPreemptionAndRestoreLifecycle verifies stateful logs cover one complete urgent cycle.
func TestUrgentManagerLogsPreemptionAndRestoreLifecycle(t *testing.T) {
	output := captureUrgentManagerLogs(t)
	manager := newTestUrgentManager(&fakeQueueController{}, 10*time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)

	_, normalDone, allowed := manager.BeginNormal(context.Background())
	require.True(t, allowed)

	urgentReady := make(chan error, 1)
	var urgentDone func()
	go func() {
		var err error
		urgentDone, err = manager.BeginUrgent(context.Background())
		urgentReady <- err
	}()
	require.Eventually(t, func() bool {
		return manager.State() == UrgentStatePreempting
	}, time.Second, time.Millisecond)
	normalDone()
	require.NoError(t, <-urgentReady)
	urgentDone()
	require.Eventually(t, func() bool {
		return manager.State() == UrgentStateNormal
	}, time.Second, time.Millisecond)
	require.Eventually(t, func() bool {
		return strings.Contains(output.String(), `"msg":"normal queue restored"`)
	}, time.Second, time.Millisecond)

	logs := output.String()
	require.Contains(t, logs, `"msg":"urgent work triggered"`)
	require.Contains(t, logs, `"msg":"preempting normal work"`)
	require.Contains(t, logs, `"msg":"normal queue removed for urgent execution"`)
	require.Contains(t, logs, `"remove_duration":`)
	require.Contains(t, logs, `"msg":"normal work drained for urgent execution"`)
	require.Contains(t, logs, `"msg":"urgent work finished"`)
	require.Contains(t, logs, `"msg":"normal queue restored"`)
	require.Contains(t, logs, `"state":"PREEMPTING"`)
	require.Contains(t, logs, `"state":"URGENT"`)
	require.Contains(t, logs, `"state":"URGENT_IDLE"`)
	require.Contains(t, logs, `"state":"NORMAL"`)
}

func TestUrgentManagerAlreadyCanceledUrgentDoesNotStartCycle(t *testing.T) {
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done, err := manager.BeginUrgent(ctx)

	require.Nil(t, done)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, UrgentStateNormal, manager.State())
	removeCalls, _ := controller.calls()
	require.Zero(t, removeCalls)
}

func TestUrgentManagerBeginNormalIsPreemptedBeforeUrgentStarts(t *testing.T) {
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)

	ctx, done, allowed := manager.BeginNormal(context.Background())
	require.True(t, allowed)

	urgentReady := make(chan error, 1)
	var urgentDone func()
	go func() {
		var err error
		urgentDone, err = manager.BeginUrgent(context.Background())
		urgentReady <- err
	}()

	require.Eventually(t, func() bool {
		return errors.Is(context.Cause(ctx), ErrUrgentPreempt)
	}, time.Second, time.Millisecond)
	select {
	case <-urgentReady:
		t.Fatal("urgent work started before normal work exited")
	default:
	}

	done()
	require.NoError(t, <-urgentReady)
	require.NotNil(t, urgentDone)
	urgentDone()

	removeCalls, _ := controller.calls()
	require.Equal(t, 1, removeCalls)
}

func TestUrgentManagerRejectsNormalDuringUrgentCycle(t *testing.T) {
	manager := newTestUrgentManager(&fakeQueueController{}, time.Hour)
	defer manager.Close(ErrWorkerShutdown)

	done, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	defer done()

	ctx, normalDone, allowed := manager.BeginNormal(context.Background())
	require.False(t, allowed)
	require.Nil(t, ctx)
	require.Nil(t, normalDone)
}

func TestUrgentManagerSharesOnePreemptionCycle(t *testing.T) {
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)

	_, normalDone, allowed := manager.BeginNormal(context.Background())
	require.True(t, allowed)

	results := make(chan struct {
		done func()
		err  error
	}, 2)
	for range 2 {
		go func() {
			done, err := manager.BeginUrgent(context.Background())
			results <- struct {
				done func()
				err  error
			}{done: done, err: err}
		}()
	}

	require.Eventually(t, func() bool {
		return manager.UrgentReservations() == 2
	}, time.Second, time.Millisecond)
	normalDone()

	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	first.done()
	second.done()

	removeCalls, _ := controller.calls()
	require.Equal(t, 1, removeCalls)
}

func TestUrgentManagerRestoresOnlyAfterLastUrgentAndIdleDelay(t *testing.T) {
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, 20*time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)

	firstDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	secondDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)

	firstDone()
	time.Sleep(40 * time.Millisecond)
	_, addCalls := controller.calls()
	require.Zero(t, addCalls)

	secondDone()
	require.Eventually(t, func() bool {
		_, addCalls := controller.calls()
		return addCalls == 1 && manager.State() == UrgentStateNormal
	}, time.Second, time.Millisecond)
}

func TestUrgentManagerRetriesNormalQueueRestoreAfterAddFailure(t *testing.T) {
	output := captureUrgentManagerLogs(t)
	controller := &fakeQueueController{
		addErr:                   errors.New("add failed"),
		addFailures:              1,
		addFailureLeavesProducer: true,
	}
	manager := newTestUrgentManager(controller, 10*time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)

	done, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	done()

	require.Eventually(t, func() bool {
		_, addCalls := controller.calls()
		return addCalls >= 2 && manager.State() == UrgentStateNormal
	}, time.Second, time.Millisecond)
	removeCalls, _ := controller.calls()
	require.GreaterOrEqual(t, removeCalls, 2)
	require.Contains(t, output.String(), `"msg":"failed to restore normal queue"`)
	require.Contains(t, output.String(), `"retry_scheduled":true`)
	require.Contains(t, output.String(), `"retry_delay":10000000`)
}

func TestUrgentManagerNewUrgentInvalidatesIdleRestore(t *testing.T) {
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, 50*time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)

	firstDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	firstDone()

	time.Sleep(10 * time.Millisecond)
	secondDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	time.Sleep(70 * time.Millisecond)
	_, addCalls := controller.calls()
	require.Zero(t, addCalls)

	secondDone()
	require.Eventually(t, func() bool {
		_, addCalls := controller.calls()
		return addCalls == 1
	}, time.Second, time.Millisecond)
}

func TestUrgentManagerCanceledLastWaiterStillFinishesAndRestoresCycle(t *testing.T) {
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, 10*time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)

	_, normalDone, allowed := manager.BeginNormal(context.Background())
	require.True(t, allowed)

	jobCtx, cancelJob := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := manager.BeginUrgent(jobCtx)
		result <- err
	}()
	require.Eventually(t, func() bool {
		return manager.UrgentReservations() == 1
	}, time.Second, time.Millisecond)

	cancelJob()
	require.ErrorIs(t, <-result, context.Canceled)
	normalDone()

	require.Eventually(t, func() bool {
		_, addCalls := controller.calls()
		return manager.State() == UrgentStateNormal && addCalls == 1
	}, time.Second, time.Millisecond)
}

func TestUrgentManagerCanceledWaiterKeepsCycleForOtherReservation(t *testing.T) {
	controller := newControlledRemoveQueueController()
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	go func() {
		_, err := manager.BeginUrgent(firstCtx)
		firstResult <- err
	}()
	<-controller.firstStarted

	secondResult := make(chan struct {
		done func()
		err  error
	}, 1)
	go func() {
		done, err := manager.BeginUrgent(context.Background())
		secondResult <- struct {
			done func()
			err  error
		}{done: done, err: err}
	}()
	require.Eventually(t, func() bool {
		return manager.UrgentReservations() == 2
	}, time.Second, time.Millisecond)

	cancelFirst()
	require.ErrorIs(t, <-firstResult, context.Canceled)
	select {
	case <-controller.firstCanceled:
		t.Fatal("shared preemption was canceled while another reservation remained")
	default:
	}

	close(controller.allowFirst)
	second := <-secondResult
	require.NoError(t, second.err)
	require.NotNil(t, second.done)
	removeCalls, addCalls := controller.calls()
	require.Equal(t, 1, removeCalls)
	require.Zero(t, addCalls)
	second.done()
}

func TestUrgentManagerCanceledFinalWaiterInterruptsAndReconcilesPreemption(t *testing.T) {
	controller := newControlledRemoveQueueController()
	manager := newTestUrgentManager(controller, time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)

	jobCtx, cancelJob := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := manager.BeginUrgent(jobCtx)
		result <- err
	}()

	<-controller.firstStarted
	cancelJob()
	require.ErrorIs(t, <-result, context.Canceled)
	select {
	case <-controller.firstCanceled:
	case <-time.After(time.Second):
		t.Fatal("preemption removal was not canceled after the final reservation left")
	}
	select {
	case <-controller.reconcileStarted:
	case <-time.After(time.Second):
		t.Fatal("normal queue removal was not reconciled")
	}

	_, normalDone, allowed := manager.BeginNormal(context.Background())
	require.False(t, allowed)
	require.Nil(t, normalDone)

	close(controller.allowReconcile)
	select {
	case <-controller.addCalled:
	case <-time.After(time.Second):
		t.Fatal("normal queue was not restored")
	}
	require.Eventually(t, func() bool {
		return manager.State() == UrgentStateNormal
	}, time.Second, time.Millisecond)
}

func TestUrgentManagerNewWaiterContinuesReconciledPreemption(t *testing.T) {
	controller := newControlledRemoveQueueController()
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	go func() {
		_, err := manager.BeginUrgent(firstCtx)
		firstResult <- err
	}()
	<-controller.firstStarted
	cancelFirst()
	require.ErrorIs(t, <-firstResult, context.Canceled)
	<-controller.reconcileStarted

	secondResult := make(chan struct {
		done func()
		err  error
	}, 1)
	go func() {
		done, err := manager.BeginUrgent(context.Background())
		secondResult <- struct {
			done func()
			err  error
		}{done: done, err: err}
	}()
	require.Eventually(t, func() bool {
		return manager.UrgentReservations() == 1
	}, time.Second, time.Millisecond)
	close(controller.allowReconcile)

	second := <-secondResult
	require.NoError(t, second.err)
	require.NotNil(t, second.done)
	_, addCalls := controller.calls()
	require.Zero(t, addCalls)
	second.done()
}

func TestUrgentManagerClosePreventsRestoreAndNewWork(t *testing.T) {
	output := captureUrgentManagerLogs(t)
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, 20*time.Millisecond)

	urgentDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	urgentDone()
	manager.Close(ErrWorkerShutdown)

	time.Sleep(50 * time.Millisecond)
	_, addCalls := controller.calls()
	require.Zero(t, addCalls)
	require.Equal(t, UrgentStateClosed, manager.State())

	_, _, allowed := manager.BeginNormal(context.Background())
	require.False(t, allowed)
	_, err = manager.BeginUrgent(context.Background())
	require.ErrorIs(t, err, ErrWorkerShutdown)
	require.Contains(t, output.String(), `"msg":"urgent manager closed"`)
	require.Contains(t, output.String(), `"state":"CLOSED"`)
	require.Contains(t, output.String(), `"urgent_reservations":0`)
	require.Contains(t, output.String(), `"normal_work_count":0`)
	require.Contains(t, output.String(), `"cause":"mirror worker shutting down"`)
}

func TestUrgentManagerRemoveFailureReturnsToNormal(t *testing.T) {
	output := captureUrgentManagerLogs(t)
	removeErr := errors.New("remove failed")
	manager := newTestUrgentManager(&fakeQueueController{removeErr: removeErr}, time.Hour)
	defer manager.Close(ErrWorkerShutdown)

	done, err := manager.BeginUrgent(context.Background())
	require.Nil(t, done)
	require.ErrorIs(t, err, removeErr)
	require.Equal(t, UrgentStateNormal, manager.State())

	ctx, normalDone, allowed := manager.BeginNormal(context.Background())
	require.True(t, allowed)
	require.NotNil(t, ctx)
	normalDone()
	require.Contains(t, output.String(), `"msg":"failed to remove normal queue for urgent execution"`)
	require.Contains(t, output.String(), `"remove_duration":`)
	require.Contains(t, output.String(), `"error":"remove failed"`)
}
