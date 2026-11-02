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

// controlledRemoveQueueController gates the initial removal and later retries or failed-Add cleanup.
type controlledRemoveQueueController struct {
	mu             sync.Mutex
	removeCalls    int
	addCalls       int
	firstError     error
	removeContexts []context.Context
	firstStarted   chan struct{}
	firstCanceled  chan struct{}
	allowFirst     chan struct{}
	nextStarted    chan struct{}
	allowNext      chan struct{}
	addCalled      chan struct{}
	addError       error
}

// newControlledRemoveQueueController creates queue synchronization gates for preemption tests.
func newControlledRemoveQueueController() *controlledRemoveQueueController {
	return &controlledRemoveQueueController{
		firstStarted:  make(chan struct{}),
		firstCanceled: make(chan struct{}),
		allowFirst:    make(chan struct{}),
		nextStarted:   make(chan struct{}),
		allowNext:     make(chan struct{}),
		addCalled:     make(chan struct{}, 1),
	}
}

// RemoveQueue records bounded attempt contexts and blocks each removal until released or canceled.
func (c *controlledRemoveQueueController) RemoveQueue(ctx context.Context, queue string) error {
	c.mu.Lock()
	c.removeCalls++
	c.removeContexts = append(c.removeContexts, ctx)
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
			return c.firstError
		}
	default:
		if call == 2 {
			close(c.nextStarted)
		}
		select {
		case <-c.allowNext:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
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
	return c.addError
}

// calls returns the recorded RemoveQueue and AddQueue call counts.
func (c *controlledRemoveQueueController) calls() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.removeCalls, c.addCalls
}

// contexts returns a snapshot of the removal contexts without exposing the mutable slice.
func (c *controlledRemoveQueueController) contexts() []context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]context.Context(nil), c.removeContexts...)
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
	done, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	done()
	require.Eventually(t, func() bool {
		return manager.State() == UrgentStateNormal && strings.Contains(output.String(), `"msg":"normal queue restored"`)
	}, time.Second, time.Millisecond)
	logs := output.String()
	for _, message := range []string{"urgent work triggered", "preempting normal work", "normal queue removed for urgent execution", "urgent work finished", "normal queue restored"} {
		require.Contains(t, logs, `"msg":"`+message+`"`)
	}
	require.Contains(t, logs, `"remove_duration":`)
	require.Contains(t, logs, `"state":"URGENT_IDLE"`)
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

// TestUrgentManagerWaitsForRiverRemoval verifies River's result barrier gates urgent execution.
func TestUrgentManagerWaitsForRiverRemoval(t *testing.T) {
	controller := newControlledRemoveQueueController()
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)
	ready := make(chan error, 1)
	go func() {
		done, err := manager.BeginUrgent(context.Background())
		if done != nil {
			defer done()
		}
		ready <- err
	}()
	<-controller.firstStarted
	require.Equal(t, UrgentStatePreempting, manager.State())
	select {
	case err := <-ready:
		t.Fatalf("urgent started before River settled normal claims: %v", err)
	default:
	}
	close(controller.allowFirst)
	require.NoError(t, <-ready)
}

// TestUrgentManagerSharesOnePreemptionCycle verifies concurrent urgent reservations share a River removal.
func TestUrgentManagerSharesOnePreemptionCycle(t *testing.T) {
	controller := newControlledRemoveQueueController()
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)

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
	close(controller.allowFirst)

	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	first.done()
	second.done()

	removeCalls, _ := controller.calls()
	require.Equal(t, 1, removeCalls)
}

// TestUrgentManagerRestoresOnlyAfterLastUrgentAndIdleDelay verifies duplicate releases cannot restore early.
func TestUrgentManagerRestoresOnlyAfterLastUrgentAndIdleDelay(t *testing.T) {
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, 20*time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)

	firstDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	secondDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)

	firstDone()
	firstDone()
	require.Equal(t, 1, manager.UrgentReservations())
	time.Sleep(40 * time.Millisecond)
	_, addCalls := controller.calls()
	require.Zero(t, addCalls)

	secondDone()
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

// TestUrgentManagerNewUrgentInvalidatesIdleRestore verifies a new reservation restarts the complete debounce.
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

	secondFinished := time.Now()
	secondDone()
	require.Eventually(t, func() bool {
		_, addCalls := controller.calls()
		return addCalls == 1
	}, time.Second, time.Millisecond)
	require.GreaterOrEqual(t, time.Since(secondFinished), 50*time.Millisecond)
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

// TestUrgentManagerCanceledFinalWaiterKeepsRemoval verifies cancellation releases only the reservation and retains one removal.
func TestUrgentManagerCanceledFinalWaiterKeepsRemoval(t *testing.T) {
	controller := newControlledRemoveQueueController()
	idleDelay := 20 * time.Millisecond
	manager := newTestUrgentManager(controller, idleDelay)
	defer manager.Close(ErrWorkerShutdown)

	jobCtx, cancelJob := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := manager.BeginUrgent(jobCtx)
		result <- err
	}()

	<-controller.firstStarted
	removeCtx := controller.contexts()[0]
	cancelJob()
	require.ErrorIs(t, <-result, context.Canceled)
	require.NoError(t, removeCtx.Err())
	require.Zero(t, manager.UrgentReservations())
	require.Equal(t, UrgentStatePreempting, manager.State())
	removeCalls, addCalls := controller.calls()
	require.Equal(t, 1, removeCalls)
	require.Zero(t, addCalls)

	settledAfter := time.Now()
	close(controller.allowFirst)
	select {
	case <-controller.addCalled:
	case <-time.After(time.Second):
		t.Fatal("normal queue was not restored")
	}
	require.Eventually(t, func() bool {
		return manager.State() == UrgentStateNormal
	}, time.Second, time.Millisecond)
	require.GreaterOrEqual(t, time.Since(settledAfter), idleDelay)
	removeCalls, addCalls = controller.calls()
	require.Equal(t, 1, removeCalls)
	require.Equal(t, 1, addCalls)
}

// TestUrgentManagerNewWaiterJoinsRemoval verifies a later urgent job reuses removal after all earlier waiters cancel.
func TestUrgentManagerNewWaiterJoinsRemoval(t *testing.T) {
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
	require.NoError(t, controller.contexts()[0].Err())

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
	select {
	case <-secondResult:
		t.Fatal("new urgent job started before removal completed")
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

// TestUrgentManagerClosePreventsRestoreAndNewWork verifies shutdown invalidates timers and rejects urgent work.
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

	_, err = manager.BeginUrgent(context.Background())
	require.ErrorIs(t, err, ErrWorkerShutdown)
	require.Contains(t, output.String(), `"msg":"urgent manager closed"`)
	require.Contains(t, output.String(), `"state":"CLOSED"`)
	require.Contains(t, output.String(), `"urgent_reservations":0`)
	require.Contains(t, output.String(), `"cause":"mirror worker shutting down"`)
}

// TestUrgentManagerRemoveFailureKeepsBarrierClosed verifies failure never reopens a stopping producer.
func TestUrgentManagerRemoveFailureKeepsBarrierClosed(t *testing.T) {
	controller := newControlledRemoveQueueController()
	controller.firstError = context.DeadlineExceeded
	manager := newTestUrgentManager(controller, 10*time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)
	ready := make(chan error, 1)
	go func() {
		done, err := manager.BeginUrgent(context.Background())
		if done != nil {
			defer done()
		}
		ready <- err
	}()
	<-controller.firstStarted
	close(controller.allowFirst)
	<-controller.nextStarted
	require.Equal(t, UrgentStatePreempting, manager.State())
	_, adds := controller.calls()
	require.Zero(t, adds)
	select {
	case <-ready:
		t.Fatal("urgent started after an incomplete removal")
	default:
	}
	close(controller.allowNext)
	require.NoError(t, <-ready)
	require.Eventually(t, func() bool { return manager.State() == UrgentStateNormal }, time.Second, time.Millisecond)
}

// TestUrgentManagerRemovalRetryUsesFreshContexts verifies timeout failures retain the barrier and retry with a fresh bounded context.
func TestUrgentManagerRemovalRetryUsesFreshContexts(t *testing.T) {
	controller := newControlledRemoveQueueController()
	controller.firstError = context.DeadlineExceeded
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)
	ready := make(chan error, 1)
	go func() {
		done, err := manager.BeginUrgent(context.Background())
		if done != nil {
			defer done()
		}
		ready <- err
	}()
	<-controller.firstStarted
	firstCtx := controller.contexts()[0]
	firstDeadline, ok := firstCtx.Deadline()
	require.True(t, ok)
	require.Positive(t, time.Until(firstDeadline))
	require.LessOrEqual(t, time.Until(firstDeadline), 30*time.Second)
	failedAfter := time.Now()
	close(controller.allowFirst)
	select {
	case <-controller.nextStarted:
	case <-time.After(normalQueueRemoveRetryDelay + time.Second):
		t.Fatal("timed-out removal was not retried")
	}
	require.GreaterOrEqual(t, time.Since(failedAfter), normalQueueRemoveRetryDelay)
	secondCtx := controller.contexts()[1]
	require.NotSame(t, firstCtx, secondCtx)
	require.ErrorIs(t, firstCtx.Err(), context.Canceled)
	require.NoError(t, secondCtx.Err())
	secondDeadline, ok := secondCtx.Deadline()
	require.True(t, ok)
	require.True(t, secondDeadline.After(firstDeadline))
	require.LessOrEqual(t, time.Until(secondDeadline), 30*time.Second)
	require.Equal(t, UrgentStatePreempting, manager.State())
	select {
	case <-ready:
		t.Fatal("urgent started before the retry settled")
	default:
	}
	close(controller.allowNext)
	require.NoError(t, <-ready)
	removes, adds := controller.calls()
	require.Equal(t, 2, removes)
	require.Zero(t, adds)
}

// TestUrgentManagerCloseCancelsRemoval verifies shutdown cancels the active attempt and releases waiting jobs with the shutdown cause.
func TestUrgentManagerCloseCancelsRemoval(t *testing.T) {
	controller := newControlledRemoveQueueController()
	manager := newTestUrgentManager(controller, time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)
	result := make(chan error, 1)
	go func() {
		_, err := manager.BeginUrgent(context.Background())
		result <- err
	}()
	<-controller.firstStarted
	removeCtx := controller.contexts()[0]
	manager.Close(ErrWorkerShutdown)
	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrWorkerShutdown)
	case <-time.After(time.Second):
		t.Fatal("urgent waiter did not exit during shutdown")
	}
	select {
	case <-controller.firstCanceled:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the active removal")
	}
	require.ErrorIs(t, removeCtx.Err(), context.Canceled)
	require.Zero(t, manager.UrgentReservations())
	require.Equal(t, UrgentStateClosed, manager.State())
	removes, adds := controller.calls()
	require.Equal(t, 1, removes)
	require.Zero(t, adds)
}

// TestUrgentManagerCloseDuringRemovalRetryDelay verifies shutdown ends the retry wait without issuing another removal.
func TestUrgentManagerCloseDuringRemovalRetryDelay(t *testing.T) {
	controller := newControlledRemoveQueueController()
	controller.firstError = errors.New("database unavailable")
	manager := newTestUrgentManager(controller, time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)
	result := make(chan error, 1)
	go func() {
		_, err := manager.BeginUrgent(context.Background())
		result <- err
	}()
	<-controller.firstStarted
	removeCtx := controller.contexts()[0]
	close(controller.allowFirst)
	select {
	case <-removeCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("failed removal did not release its attempt context")
	}
	manager.Close(ErrWorkerShutdown)
	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrWorkerShutdown)
	case <-time.After(time.Second):
		t.Fatal("shutdown did not interrupt the retry delay")
	}
	removes, adds := controller.calls()
	require.Equal(t, 1, removes)
	require.Zero(t, adds)
}

// TestUrgentManagerAbsentQueueIsSettled verifies an already removed producer admits urgent work and can later be added.
func TestUrgentManagerAbsentQueueIsSettled(t *testing.T) {
	controller := &fakeQueueController{removeErr: &river.QueueNotFoundError{Name: MirrorRepoQueue}}
	manager := newTestUrgentManager(controller, time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)
	done, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	done()
	require.Eventually(t, func() bool { return manager.State() == UrgentStateNormal }, time.Second, time.Millisecond)
	removes, adds := controller.calls()
	require.Equal(t, 1, removes)
	require.Equal(t, 1, adds)
}

// TestUrgentManagerCloseInterruptsFailedRestoreCleanup cancels restoration cleanup promptly.
func TestUrgentManagerCloseInterruptsFailedRestoreCleanup(t *testing.T) {
	controller := &fakeQueueController{addErr: errors.New("start failed"), addFailures: 1}
	manager := newTestUrgentManager(controller, time.Millisecond)
	done, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	controller.mu.Lock()
	controller.removeErr = errors.New("database unavailable")
	controller.mu.Unlock()
	done()
	require.Eventually(t, func() bool {
		removes, adds := controller.calls()
		return adds == 1 && removes >= 2
	}, time.Second, time.Millisecond)
	closed := make(chan struct{})
	go func() { manager.Close(ErrWorkerShutdown); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close blocked behind removal cleanup")
	}
	require.Equal(t, UrgentStateClosed, manager.State())
	_, adds := controller.calls()
	require.Equal(t, 1, adds)
}

// TestUrgentManagerStaleTimerCannotRestore verifies generation checks reject an already queued callback.
func TestUrgentManagerStaleTimerCannotRestore(t *testing.T) {
	controller := &fakeQueueController{}
	manager := newTestUrgentManager(controller, time.Hour)
	defer manager.Close(ErrWorkerShutdown)
	done, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	done()
	manager.mu.Lock()
	oldGeneration := manager.timerGeneration
	manager.mu.Unlock()
	done, err = manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	done()
	manager.restoreNormal(oldGeneration)
	_, adds := controller.calls()
	require.Zero(t, adds)
}

// TestUrgentManagerNewUrgentWaitsForFailedAddCleanup reuses the barrier without blocking reservation cancellation.
func TestUrgentManagerNewUrgentWaitsForFailedAddCleanup(t *testing.T) {
	controller := newControlledRemoveQueueController()
	controller.addError = errors.New("producer start failed")
	close(controller.allowFirst)
	manager := newTestUrgentManager(controller, time.Millisecond)
	defer manager.Close(ErrWorkerShutdown)
	done, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	done()
	<-controller.nextStarted

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		done, err := manager.BeginUrgent(ctx)
		if done != nil {
			defer done()
		}
		result <- err
	}()
	require.Eventually(t, func() bool { return manager.UrgentReservations() == 1 }, time.Second, time.Millisecond)
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("reservation cancellation blocked behind failed Add cleanup")
	}
	_, adds := controller.calls()
	require.Equal(t, 1, adds)
	require.Equal(t, UrgentStatePreempting, manager.State())
}
