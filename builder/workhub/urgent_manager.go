package workhub

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/riverqueue/river"
)

// ErrWorkerShutdown indicates that the owning work client is shutting down.
var ErrWorkerShutdown = errors.New("mirror worker shutting down")

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
	urgentReservations int
	// cycle closes only after River confirms removal; shutdown is signaled by managerCtx.
	cycle           chan struct{}
	idleTimer       *time.Timer
	timerGeneration uint64
}

// NewUrgentManager creates a manager in the NORMAL state.
func NewUrgentManager(config UrgentManagerConfig) *UrgentManager {
	managerCtx, cancelManager := context.WithCancelCause(context.Background())
	return &UrgentManager{
		queueController:   config.QueueController,
		normalQueue:       config.NormalQueue,
		normalQueueConfig: config.NormalQueueConfig,
		urgentIdleDelay:   config.UrgentIdleDelay,
		managerCtx:        managerCtx,
		cancelManager:     cancelManager,
		state:             UrgentStateNormal,
	}
}

// BeginUrgent reserves urgent capacity and waits for the shared preemption barrier.
func (m *UrgentManager) BeginUrgent(jobCtx context.Context) (func(), error) {
	if err := context.Cause(jobCtx); err != nil {
		return nil, err
	}
	m.transitionMu.Lock()
	if err := context.Cause(jobCtx); err != nil {
		m.transitionMu.Unlock()
		return nil, err
	}
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
	case <-cycle:
		if err := context.Cause(m.managerCtx); err != nil {
			done()
			return nil, err
		}
		if err := context.Cause(jobCtx); err != nil {
			done()
			return nil, err
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
func (m *UrgentManager) reserveUrgent() (chan struct{}, bool, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := context.Cause(m.managerCtx); err != nil {
		return nil, false, nil, err
	}

	m.urgentReservations++
	m.timerGeneration++
	if m.idleTimer != nil {
		m.idleTimer.Stop()
		m.idleTimer = nil
	}

	var cycle chan struct{}
	shouldStart := false
	switch m.state {
	case UrgentStateNormal:
		cycle = m.newPreemptionCycleLocked()
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
		slog.Bool("starts_preemption", shouldStart),
	)

	var once sync.Once
	done := func() {
		once.Do(m.releaseUrgent)
	}
	return cycle, shouldStart, done, nil
}

// newPreemptionCycleLocked gates urgent work until the normal producer is fully removed; m.mu must be held.
func (m *UrgentManager) newPreemptionCycleLocked() chan struct{} {
	cycle := make(chan struct{})
	m.cycle = cycle
	m.state = UrgentStatePreempting
	return cycle
}

// runPreemptionCycle waits for River to interrupt and settle local normal claims before admitting urgent work.
func (m *UrgentManager) runPreemptionCycle(cycle chan struct{}) {
	slog.Info("preempting normal work", slog.String("queue", m.normalQueue))
	started := time.Now()
	if err := m.removeNormalQueue(); err != nil {
		slog.Error("failed to remove normal queue for urgent execution",
			slog.String("queue", m.normalQueue), slog.Duration("remove_duration", time.Since(started)), slog.Any("error", err))
		return
	}

	m.mu.Lock()
	if m.cycle == cycle && m.state != UrgentStateClosed && m.managerCtx.Err() == nil {
		m.state = UrgentStateUrgent
		if m.urgentReservations == 0 {
			m.startIdleTimerLocked()
		}
		close(cycle)
	}
	state := m.state
	reservations := m.urgentReservations
	m.mu.Unlock()

	slog.Info("normal queue removed for urgent execution",
		slog.String("queue", m.normalQueue), slog.String("state", string(state)),
		slog.Int("urgent_reservations", reservations), slog.Duration("remove_duration", time.Since(started)))
}

const (
	// normalQueueRemoveTimeout bounds each River removal attempt, including claim settlement.
	normalQueueRemoveTimeout = 30 * time.Second
	// normalQueueRemoveRetryDelay prevents a failed producer removal from spinning.
	normalQueueRemoveRetryDelay = time.Second
)

// removeNormalQueue retries bounded attempts until River confirms removal or the manager closes.
// Urgent waiter cancellation does not interrupt this operation because it cannot undo a removal already started.
// An absent producer is already settled.
func (m *UrgentManager) removeNormalQueue() error {
	for {
		if err := context.Cause(m.managerCtx); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(m.managerCtx, normalQueueRemoveTimeout)
		err := m.queueController.RemoveQueue(ctx, m.normalQueue)
		cancel()
		var notFoundErr *river.QueueNotFoundError
		if err == nil || errors.As(err, &notFoundErr) {
			return nil
		}
		slog.Warn("normal queue removal remains incomplete", slog.String("queue", m.normalQueue), slog.Any("error", err))
		timer := time.NewTimer(normalQueueRemoveRetryDelay)
		select {
		case <-m.managerCtx.Done():
			timer.Stop()
			return context.Cause(m.managerCtx)
		case <-timer.C:
		}
	}
}

// releaseUrgent releases one reservation and schedules normal queue restoration after the final urgent job.
func (m *UrgentManager) releaseUrgent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.urgentReservations == 0 {
		return
	}
	m.urgentReservations--
	slog.Info("urgent work finished",
		slog.String("queue", m.normalQueue),
		slog.String("state", string(m.state)),
		slog.Int("urgent_reservations", m.urgentReservations),
	)
	// An in-progress removal owns its own completion and schedules restoration if no waiters remain.
	if m.urgentReservations == 0 && m.state == UrgentStateUrgent {
		m.startIdleTimerLocked()
	}
}

// startIdleTimerLocked enters URGENT_IDLE and schedules queue restoration while m.mu is held.
func (m *UrgentManager) startIdleTimerLocked() {
	m.state = UrgentStateUrgentIdle
	slog.Info("normal queue restore scheduled",
		slog.String("queue", m.normalQueue),
		slog.String("state", string(m.state)),
		slog.Int("urgent_reservations", m.urgentReservations),
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
	valid := m.state == UrgentStateUrgentIdle && m.managerCtx.Err() == nil &&
		m.urgentReservations == 0 &&
		generation == m.timerGeneration
	m.mu.Unlock()
	if !valid {
		return
	}

	if err := m.queueController.AddQueue(m.normalQueue, m.normalQueueConfig); err != nil {
		// A failed Add may leave a registered producer. Reuse the removal barrier
		// without holding transitionMu during database retries or blocking new reservations.
		m.mu.Lock()
		var cycle chan struct{}
		if m.state == UrgentStateUrgentIdle && m.managerCtx.Err() == nil &&
			m.urgentReservations == 0 && generation == m.timerGeneration {
			m.timerGeneration++
			m.idleTimer = nil
			cycle = m.newPreemptionCycleLocked()
		}
		m.mu.Unlock()
		if cycle != nil {
			go m.runPreemptionCycle(cycle)
		}
		slog.Error("failed to restore normal queue",
			slog.String("queue", m.normalQueue), slog.Bool("retry_scheduled", cycle != nil),
			slog.Duration("retry_delay", m.urgentIdleDelay), slog.Any("error", err))
		return
	}

	m.mu.Lock()
	restored := false
	if m.state == UrgentStateUrgentIdle && m.managerCtx.Err() == nil &&
		m.urgentReservations == 0 &&
		generation == m.timerGeneration {
		m.cycle = nil
		m.idleTimer = nil
		m.state = UrgentStateNormal
		restored = true
	}
	state := m.state
	urgentReservations := m.urgentReservations
	m.mu.Unlock()
	if restored {
		slog.Info("normal queue restored",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(state)),
			slog.Int("urgent_reservations", urgentReservations),
		)
	}
}

// Close rejects new work and prevents normal queue restoration.
func (m *UrgentManager) Close(cause error) {
	if cause == nil {
		cause = ErrWorkerShutdown
	}
	m.closeOnce.Do(func() {
		// Unblock removal retries before waiting for a restoration holding transitionMu.
		m.cancelManager(cause)
		m.transitionMu.Lock()
		m.mu.Lock()
		previousState := m.state
		m.state = UrgentStateClosed
		m.timerGeneration++
		timer := m.idleTimer
		cycle := m.cycle
		urgentReservations := m.urgentReservations
		queueRemoved := previousState == UrgentStateUrgent || previousState == UrgentStateUrgentIdle
		m.idleTimer = nil
		m.mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		m.transitionMu.Unlock()
		slog.Info("urgent manager closed",
			slog.String("queue", m.normalQueue),
			slog.String("state", string(UrgentStateClosed)),
			slog.String("previous_state", string(previousState)),
			slog.Int("urgent_reservations", urgentReservations),
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
