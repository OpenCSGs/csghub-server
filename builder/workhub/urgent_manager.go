package workhub

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/riverqueue/river"
)

var (
	// ErrUrgentPreempt indicates that normal mirror work yielded to urgent work.
	ErrUrgentPreempt = errors.New("mirror work preempted by urgent job")
	// ErrWorkerShutdown indicates that the owning work client is shutting down.
	ErrWorkerShutdown = errors.New("mirror worker shutting down")
)

// UrgentState is the process-local execution state for one mirror work client.
type UrgentState string

const (
	// UrgentStateNormal allows normal work to start.
	UrgentStateNormal UrgentState = "NORMAL"
	// UrgentStatePreempting cancels and drains normal work before urgent work starts.
	UrgentStatePreempting UrgentState = "PREEMPTING"
	// UrgentStateUrgent allows urgent work to run while normal work remains disabled.
	UrgentStateUrgent UrgentState = "URGENT"
	// UrgentStateUrgentIdle waits briefly before restoring normal work.
	UrgentStateUrgentIdle UrgentState = "URGENT_IDLE"
	// UrgentStateClosed rejects new work while existing work cleans up.
	UrgentStateClosed UrgentState = "CLOSED"
)

// localQueueController changes queue producers only on the current River client.
type localQueueController interface {
	RemoveQueue(ctx context.Context, queue string) error
	AddQueue(queue string, config river.QueueConfig) error
}

// UrgentManagerConfig configures one process-local urgent execution manager.
type UrgentManagerConfig struct {
	// QueueController controls queues on the owning River client.
	QueueController localQueueController
	// NormalQueue is the queue suspended while urgent work is active.
	NormalQueue string
	// NormalQueueConfig is reused when restoring the normal queue.
	NormalQueueConfig river.QueueConfig
	// UrgentIdleDelay is the quiet period before normal work resumes.
	UrgentIdleDelay time.Duration
}

type normalRegistration struct {
	cancel context.CancelCauseFunc
}

type preemptionCycle struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	barrier chan struct{}
	err     error
}

// UrgentManager coordinates normal and urgent work within one River work client.
type UrgentManager struct {
	mu sync.Mutex
	// transitionMu serializes normal queue restoration with new urgent reservations.
	transitionMu sync.Mutex

	queueController   localQueueController
	normalQueue       string
	normalQueueConfig river.QueueConfig
	urgentIdleDelay   time.Duration

	managerCtx    context.Context
	cancelManager context.CancelCauseFunc
	closeOnce     sync.Once

	state              UrgentState
	normals            map[uint64]normalRegistration
	nextNormalID       uint64
	normalDrained      chan struct{}
	urgentReservations int
	cycle              *preemptionCycle
	queueRemoved       bool
	idleTimer          *time.Timer
	timerGeneration    uint64
}

// NewUrgentManager creates a manager in the NORMAL state.
func NewUrgentManager(config UrgentManagerConfig) *UrgentManager {
	managerCtx, cancelManager := context.WithCancelCause(context.Background())
	drained := make(chan struct{})
	close(drained)
	return &UrgentManager{
		queueController:   config.QueueController,
		normalQueue:       config.NormalQueue,
		normalQueueConfig: config.NormalQueueConfig,
		urgentIdleDelay:   config.UrgentIdleDelay,
		managerCtx:        managerCtx,
		cancelManager:     cancelManager,
		state:             UrgentStateNormal,
		normals:           make(map[uint64]normalRegistration),
		normalDrained:     drained,
	}
}

// BeginNormal atomically admits and registers one normal work execution.
func (m *UrgentManager) BeginNormal(riverCtx context.Context) (context.Context, func(), bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != UrgentStateNormal {
		return nil, nil, false
	}

	ctx, cancel := context.WithCancelCause(riverCtx)
	if len(m.normals) == 0 {
		m.normalDrained = make(chan struct{})
	}
	m.nextNormalID++
	id := m.nextNormalID
	m.normals[id] = normalRegistration{cancel: cancel}

	var once sync.Once
	done := func() {
		once.Do(func() {
			cancel(nil)
			m.finishNormal(id)
		})
	}
	return ctx, done, true
}

// finishNormal unregisters one normal execution and closes the drain barrier when none remain.
func (m *UrgentManager) finishNormal(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.normals[id]; !ok {
		return
	}
	delete(m.normals, id)
	if len(m.normals) == 0 {
		close(m.normalDrained)
	}
}

// BeginUrgent reserves urgent capacity and waits for the shared preemption barrier.
func (m *UrgentManager) BeginUrgent(jobCtx context.Context) (func(), error) {
	if err := context.Cause(jobCtx); err != nil {
		return nil, err
	}
	m.transitionMu.Lock()
	cycle, shouldStart, done, err := m.reserveUrgent()
	m.transitionMu.Unlock()
	if err != nil {
		return nil, err
	}
	if shouldStart {
		go m.runPreemptionCycle(cycle)
	}
	if cycle == nil {
		return done, nil
	}

	select {
	case <-cycle.barrier:
		if cycle.err != nil {
			done()
			return nil, cycle.err
		}
		return done, nil
	case <-jobCtx.Done():
		done()
		return nil, context.Cause(jobCtx)
	case <-m.managerCtx.Done():
		done()
		return nil, context.Cause(m.managerCtx)
	}
}

// reserveUrgent records one urgent reservation and returns the shared preemption cycle and release callback.
func (m *UrgentManager) reserveUrgent() (*preemptionCycle, bool, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == UrgentStateClosed {
		return nil, false, nil, context.Cause(m.managerCtx)
	}

	m.urgentReservations++
	m.timerGeneration++
	if m.idleTimer != nil {
		m.idleTimer.Stop()
		m.idleTimer = nil
	}

	var cycle *preemptionCycle
	shouldStart := false
	switch m.state {
	case UrgentStateNormal:
		cycleCtx, cancelCycle := context.WithCancelCause(m.managerCtx)
		cycle = &preemptionCycle{
			ctx:     cycleCtx,
			cancel:  cancelCycle,
			barrier: make(chan struct{}),
		}
		m.cycle = cycle
		m.state = UrgentStatePreempting
		shouldStart = true
	case UrgentStatePreempting:
		cycle = m.cycle
	case UrgentStateUrgentIdle:
		m.state = UrgentStateUrgent
	case UrgentStateUrgent:
	}
	slog.Info("urgent work triggered",
		slog.String("queue", m.normalQueue),
		slog.String("state", string(m.state)),
		slog.Int("urgent_reservations", m.urgentReservations),
		slog.Int("normal_work_count", len(m.normals)),
		slog.Bool("starts_preemption", shouldStart),
	)

	var once sync.Once
	done := func() {
		once.Do(m.releaseUrgent)
	}
	return cycle, shouldStart, done, nil
}

// runPreemptionCycle cancels normal work, removes its queue, and opens the urgent execution barrier.
func (m *UrgentManager) runPreemptionCycle(cycle *preemptionCycle) {
	m.mu.Lock()
	cancels := make([]context.CancelCauseFunc, 0, len(m.normals))
	for _, normal := range m.normals {
		cancels = append(cancels, normal.cancel)
	}
	drained := m.normalDrained
	state := m.state
	urgentReservations := m.urgentReservations
	m.mu.Unlock()
	slog.Info("preempting normal work",
		slog.String("queue", m.normalQueue),
		slog.String("state", string(state)),
		slog.Int("urgent_reservations", urgentReservations),
		slog.Int("normal_work_count", len(cancels)),
	)

	for _, cancel := range cancels {
		cancel(ErrUrgentPreempt)
	}

	removeStartedAt := time.Now()
	err := m.queueController.RemoveQueue(cycle.ctx, m.normalQueue)
	removeDuration := time.Since(removeStartedAt)
	m.mu.Lock()
	removeState := m.state
	removeReservations := m.urgentReservations
	removeNormalWorkCount := len(m.normals)
	m.mu.Unlock()
	if err != nil {
		slog.Error("failed to remove normal queue for urgent execution",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(removeState)),
			slog.Int("urgent_reservations", removeReservations),
			slog.Int("normal_work_count", removeNormalWorkCount),
			slog.Duration("remove_duration", removeDuration),
			slog.String("error", err.Error()),
		)
	} else {
		slog.Info("normal queue removed for urgent execution",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(removeState)),
			slog.Int("urgent_reservations", removeReservations),
			slog.Int("normal_work_count", removeNormalWorkCount),
			slog.Duration("remove_duration", removeDuration),
		)
	}
	if err == nil {
		select {
		case <-drained:
			m.mu.Lock()
			drainedState := m.state
			drainedReservations := m.urgentReservations
			drainedNormalWorkCount := len(m.normals)
			m.mu.Unlock()
			slog.Info("normal work drained for urgent execution",
				slog.String("queue", m.normalQueue),
				slog.String("state", string(drainedState)),
				slog.Int("urgent_reservations", drainedReservations),
				slog.Int("normal_work_count", drainedNormalWorkCount),
			)
		case <-cycle.ctx.Done():
			err = context.Cause(cycle.ctx)
		}
	}
	abandoned := errors.Is(context.Cause(cycle.ctx), context.Canceled) && context.Cause(m.managerCtx) == nil
	if abandoned {
		err = m.reconcileNormalQueueRemoval()
		if err == nil {
			select {
			case <-drained:
			case <-m.managerCtx.Done():
				err = context.Cause(m.managerCtx)
			}
		}
	}

	m.mu.Lock()
	transitioned := false
	if m.cycle == cycle {
		cycle.err = err
		if err == nil && m.state != UrgentStateClosed {
			m.queueRemoved = true
			m.state = UrgentStateUrgent
			transitioned = true
			if m.urgentReservations == 0 {
				m.startIdleTimerLocked()
			}
		} else if err != nil && m.state != UrgentStateClosed {
			m.cycle = nil
			m.queueRemoved = false
			m.state = UrgentStateNormal
		}
		close(cycle.barrier)
	}
	state = m.state
	urgentReservations = m.urgentReservations
	normalWorkCount := len(m.normals)
	m.mu.Unlock()
	if err != nil {
		slog.Error("urgent work preemption failed",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(state)),
			slog.Int("urgent_reservations", urgentReservations),
			slog.Int("normal_work_count", normalWorkCount),
			slog.String("error", err.Error()),
		)
	} else if transitioned {
		slog.Info("urgent execution ready",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(state)),
			slog.Int("urgent_reservations", urgentReservations),
			slog.Int("normal_work_count", normalWorkCount),
		)
	}
}

// reconcileNormalQueueRemoval completes a River queue removal interrupted by an abandoned preemption cycle.
func (m *UrgentManager) reconcileNormalQueueRemoval() error {
	err := m.queueController.RemoveQueue(m.managerCtx, m.normalQueue)
	var notFoundErr *river.QueueNotFoundError
	if errors.As(err, &notFoundErr) {
		return nil
	}
	return err
}

// releaseUrgent releases one reservation and schedules normal queue restoration after the final urgent job.
func (m *UrgentManager) releaseUrgent() {
	m.mu.Lock()
	if m.urgentReservations == 0 {
		m.mu.Unlock()
		return
	}
	m.urgentReservations--
	slog.Info("urgent work finished",
		slog.String("queue", m.normalQueue),
		slog.String("state", string(m.state)),
		slog.Int("urgent_reservations", m.urgentReservations),
		slog.Int("normal_work_count", len(m.normals)),
	)
	if m.urgentReservations != 0 || m.state == UrgentStateClosed {
		m.mu.Unlock()
		return
	}
	if m.state == UrgentStatePreempting {
		cycle := m.cycle
		m.mu.Unlock()
		m.cancelAbandonedPreemption(cycle)
		return
	}
	if m.state != UrgentStateUrgent {
		m.mu.Unlock()
		return
	}

	m.startIdleTimerLocked()
	m.mu.Unlock()
}

// cancelAbandonedPreemption cancels the current cycle only if no newer urgent reservation still needs it.
func (m *UrgentManager) cancelAbandonedPreemption(cycle *preemptionCycle) {
	if cycle == nil {
		return
	}
	m.transitionMu.Lock()
	m.mu.Lock()
	shouldCancel := m.cycle == cycle &&
		m.state == UrgentStatePreempting &&
		m.urgentReservations == 0
	m.mu.Unlock()
	if shouldCancel {
		cycle.cancel(context.Canceled)
	}
	m.transitionMu.Unlock()
}

// startIdleTimerLocked enters URGENT_IDLE and schedules queue restoration while m.mu is held.
func (m *UrgentManager) startIdleTimerLocked() {
	m.state = UrgentStateUrgentIdle
	slog.Info("normal queue restore scheduled",
		slog.String("queue", m.normalQueue),
		slog.String("state", string(m.state)),
		slog.Int("urgent_reservations", m.urgentReservations),
		slog.Int("normal_work_count", len(m.normals)),
		slog.Duration("idle_delay", m.urgentIdleDelay),
	)
	m.timerGeneration++
	generation := m.timerGeneration
	delay := m.urgentIdleDelay
	m.idleTimer = time.AfterFunc(delay, func() {
		m.restoreNormal(generation)
	})
}

// restoreNormal re-adds the normal queue when the idle timer generation is still current.
func (m *UrgentManager) restoreNormal(generation uint64) {
	m.transitionMu.Lock()
	defer m.transitionMu.Unlock()

	m.mu.Lock()
	valid := m.state == UrgentStateUrgentIdle &&
		m.urgentReservations == 0 &&
		generation == m.timerGeneration
	m.mu.Unlock()
	if !valid {
		return
	}

	if err := m.queueController.AddQueue(m.normalQueue, m.normalQueueConfig); err != nil {
		// River registers a producer before starting it, so a failed Add can leave a stopped producer behind.
		if cleanupErr := m.reconcileNormalQueueRemoval(); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		m.mu.Lock()
		state := m.state
		urgentReservations := m.urgentReservations
		normalWorkCount := len(m.normals)
		retryScheduled := false
		if m.state == UrgentStateUrgentIdle &&
			m.urgentReservations == 0 &&
			generation == m.timerGeneration {
			m.idleTimer = time.AfterFunc(m.urgentIdleDelay, func() {
				m.restoreNormal(generation)
			})
			retryScheduled = true
		}
		m.mu.Unlock()
		slog.Error("failed to restore normal queue",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(state)),
			slog.Int("urgent_reservations", urgentReservations),
			slog.Int("normal_work_count", normalWorkCount),
			slog.Bool("retry_scheduled", retryScheduled),
			slog.Duration("retry_delay", m.urgentIdleDelay),
			slog.String("error", err.Error()),
		)
		return
	}

	m.mu.Lock()
	restored := false
	if m.state == UrgentStateUrgentIdle &&
		m.urgentReservations == 0 &&
		generation == m.timerGeneration {
		m.queueRemoved = false
		m.cycle = nil
		m.idleTimer = nil
		m.state = UrgentStateNormal
		restored = true
	}
	state := m.state
	urgentReservations := m.urgentReservations
	normalWorkCount := len(m.normals)
	m.mu.Unlock()
	if restored {
		slog.Info("normal queue restored",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(state)),
			slog.Int("urgent_reservations", urgentReservations),
			slog.Int("normal_work_count", normalWorkCount),
		)
	}
}

// Close rejects new work and prevents normal queue restoration.
func (m *UrgentManager) Close(cause error) {
	if cause == nil {
		cause = ErrWorkerShutdown
	}
	m.closeOnce.Do(func() {
		m.transitionMu.Lock()
		m.mu.Lock()
		previousState := m.state
		m.state = UrgentStateClosed
		m.timerGeneration++
		timer := m.idleTimer
		cycle := m.cycle
		urgentReservations := m.urgentReservations
		normalWorkCount := len(m.normals)
		queueRemoved := m.queueRemoved
		m.idleTimer = nil
		m.mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		if cycle != nil {
			cycle.cancel(cause)
		}
		m.cancelManager(cause)
		m.transitionMu.Unlock()
		slog.Info("urgent manager closed",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(UrgentStateClosed)),
			slog.String("previous_state", string(previousState)),
			slog.Int("urgent_reservations", urgentReservations),
			slog.Int("normal_work_count", normalWorkCount),
			slog.Bool("queue_removed", queueRemoved),
			slog.Bool("idle_timer_active", timer != nil),
			slog.Bool("preemption_cycle_active", cycle != nil),
			slog.String("cause", cause.Error()),
		)
	})
}

// State returns the current manager state for observability and tests.
func (m *UrgentManager) State() UrgentState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// UrgentReservations returns waiting and running urgent work count.
func (m *UrgentManager) UrgentReservations() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.urgentReservations
}
