package availability

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeMultimodalProbeStateCache struct {
	mu       sync.Mutex
	states   map[int64]multimodalProbeState
	getCalls int
	getErr   error
	setErr   error
}

func (c *fakeMultimodalProbeStateCache) Enabled() bool {
	return true
}

func (c *fakeMultimodalProbeStateCache) GetMultimodalProbeState(_ context.Context, upstreamID int64) (multimodalProbeState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getCalls++
	if c.getErr != nil {
		return multimodalProbeState{}, c.getErr
	}
	state, ok := c.states[upstreamID]
	if !ok {
		return multimodalProbeState{}, errStateCacheMiss
	}
	return state, nil
}

func (c *fakeMultimodalProbeStateCache) SetMultimodalProbeState(
	_ context.Context,
	upstreamID int64,
	state multimodalProbeState,
	_ time.Duration,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.setErr != nil {
		return c.setErr
	}
	if c.states == nil {
		c.states = make(map[int64]multimodalProbeState)
	}
	c.states[upstreamID] = state
	return nil
}

func (c *fakeMultimodalProbeStateCache) TryReserveMultimodalInference(
	_ context.Context,
	upstreamID int64,
	state multimodalProbeState,
	now time.Time,
	reservedUntil time.Time,
	_ time.Duration,
) (multimodalProbeState, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.states[upstreamID]
	if !ok {
		current = state
	}
	if now.Before(current.nextInferenceAt) {
		return current, false, nil
	}
	current.nextInferenceAt = reservedUntil
	if c.states == nil {
		c.states = make(map[int64]multimodalProbeState)
	}
	c.states[upstreamID] = current
	return current, true, nil
}

func (c *fakeMultimodalProbeStateCache) DeleteMultimodalProbeState(_ context.Context, upstreamID int64) error {
	c.mu.Lock()
	delete(c.states, upstreamID)
	c.mu.Unlock()
	return nil
}

func TestMultimodalProbeScheduler_PlansInferenceByModeAndDeadline(t *testing.T) {
	start := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	policy := multimodalProbePolicy{
		retryInterval:     time.Minute,
		inferenceInterval: time.Hour,
		failureThreshold:  3,
	}
	var scheduler multimodalProbeScheduler

	plan := scheduler.plan(1, start)
	require.False(t, plan.skipAll)
	require.True(t, plan.inferenceDue)

	require.NoError(t, scheduler.recordInferenceSchedule(context.Background(), 1, start, true, 0, policy, 2*time.Hour))
	state, ok := scheduler.snapshot(1)
	require.True(t, ok)
	require.Equal(t, multimodalProbeInferenceOnly, state.mode)
	require.Equal(t, start.Add(time.Hour), state.nextInferenceAt)

	plan = scheduler.plan(1, start.Add(time.Minute))
	require.True(t, plan.skipAll)
	require.False(t, plan.inferenceDue)

	plan = scheduler.plan(1, start.Add(time.Hour))
	require.False(t, plan.skipAll)
	require.True(t, plan.inferenceDue)

	require.NoError(t, scheduler.recordL7Available(context.Background(), 1, 2*time.Hour))
	state, ok = scheduler.snapshot(1)
	require.True(t, ok)
	require.Equal(t, multimodalProbeL7Available, state.mode)
	require.Equal(t, start.Add(time.Hour), state.nextInferenceAt)
}

func TestMultimodalProbeScheduler_KeepsInferenceOnlyModeLocallyWhenSharedWriteFails(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	shared := &fakeMultimodalProbeStateCache{setErr: errTestSentinel}
	scheduler := multimodalProbeScheduler{
		stateCache: shared,
		states: map[int64]multimodalProbeState{
			1: {mode: multimodalProbeL7Available, nextInferenceAt: now.Add(time.Hour)},
		},
	}

	err := scheduler.recordInferenceOnly(context.Background(), 1, 2*time.Hour)
	require.ErrorIs(t, err, errTestSentinel)
	state, ok := scheduler.snapshot(1)
	require.True(t, ok)
	require.Equal(t, multimodalProbeInferenceOnly, state.mode)
	require.Equal(t, now.Add(time.Hour), state.nextInferenceAt)
}

func TestMultimodalProbeScheduler_RetriesFailuresThenThrottles(t *testing.T) {
	start := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	policy := multimodalProbePolicy{
		retryInterval:     time.Minute,
		inferenceInterval: time.Hour,
		failureThreshold:  3,
	}
	var scheduler multimodalProbeScheduler

	require.NoError(t, scheduler.recordInferenceSchedule(context.Background(), 1, start, false, 1, policy, 2*time.Hour))
	state, ok := scheduler.snapshot(1)
	require.True(t, ok)
	require.Equal(t, start.Add(time.Minute), state.nextInferenceAt)

	require.NoError(t, scheduler.recordInferenceSchedule(context.Background(), 1, start.Add(2*time.Minute), false, 3, policy, 2*time.Hour))
	state, ok = scheduler.snapshot(1)
	require.True(t, ok)
	require.Equal(t, start.Add(62*time.Minute), state.nextInferenceAt)

	require.NoError(t, scheduler.recordInferenceSchedule(context.Background(), 1, start.Add(62*time.Minute), false, 0, policy, 2*time.Hour))
	state, ok = scheduler.snapshot(1)
	require.True(t, ok)
	require.Equal(t, start.Add(122*time.Minute), state.nextInferenceAt)

	require.NoError(t, scheduler.recordInferenceSchedule(context.Background(), 2, start, true, 1, policy, 2*time.Hour))
	state, ok = scheduler.snapshot(2)
	require.True(t, ok)
	require.Equal(t, multimodalProbeInferenceOnly, state.mode)
	plan := scheduler.plan(2, start.Add(30*time.Second))
	require.True(t, plan.skipAll)
	plan = scheduler.plan(2, start.Add(time.Minute))
	require.False(t, plan.skipAll)
	require.True(t, plan.inferenceDue)
}

func TestMultimodalProbeScheduler_CleansInactiveUpstreams(t *testing.T) {
	start := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	policy := multimodalProbePolicy{
		retryInterval:     time.Minute,
		inferenceInterval: time.Hour,
		failureThreshold:  3,
	}
	var scheduler multimodalProbeScheduler
	require.NoError(t, scheduler.recordInferenceSchedule(context.Background(), 1, start, true, 0, policy, 2*time.Hour))
	require.NoError(t, scheduler.recordInferenceSchedule(context.Background(), 2, start, false, 0, policy, 2*time.Hour))

	scheduler.cleanup([]int64{1})

	_, firstExists := scheduler.snapshot(1)
	_, secondExists := scheduler.snapshot(2)
	require.True(t, firstExists)
	require.False(t, secondExists)
}

func TestMultimodalProbeScheduler_ReadsSharedStateOnceUntilReset(t *testing.T) {
	start := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	shared := &fakeMultimodalProbeStateCache{states: map[int64]multimodalProbeState{
		1: {mode: multimodalProbeL7Available, nextInferenceAt: start.Add(time.Hour)},
	}}
	scheduler := multimodalProbeScheduler{stateCache: shared}

	state, found, err := scheduler.load(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, start.Add(time.Hour), state.nextInferenceAt)
	_, _, err = scheduler.load(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, shared.getCalls)

	scheduler.reset()
	_, _, err = scheduler.load(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 2, shared.getCalls)
}

func TestMultimodalProbeScheduler_ReservesInferenceAcrossLeaders(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	shared := &fakeMultimodalProbeStateCache{states: map[int64]multimodalProbeState{
		1: {mode: multimodalProbeL7Available, nextInferenceAt: now},
	}}
	firstLeader := multimodalProbeScheduler{stateCache: shared}
	secondLeader := multimodalProbeScheduler{stateCache: shared}
	for _, scheduler := range []*multimodalProbeScheduler{&firstLeader, &secondLeader} {
		_, found, err := scheduler.load(context.Background(), 1)
		require.NoError(t, err)
		require.True(t, found)
	}

	reservedUntil := now.Add(6 * time.Minute)
	reserved, err := firstLeader.reserveInference(context.Background(), 1, now, reservedUntil, 2*time.Hour)
	require.NoError(t, err)
	require.True(t, reserved)

	reserved, err = secondLeader.reserveInference(context.Background(), 1, now, reservedUntil, 2*time.Hour)
	require.NoError(t, err)
	require.False(t, reserved)
	state, ok := secondLeader.snapshot(1)
	require.True(t, ok)
	require.Equal(t, reservedUntil, state.nextInferenceAt)
}

func TestMultimodalProbeScheduler_KeepsInferenceOutcomeLocallyWhenFinalWriteFails(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	reservedUntil := now.Add(6 * time.Minute)
	policy := multimodalProbePolicy{retryInterval: time.Minute, inferenceInterval: time.Hour, failureThreshold: 3}

	for _, test := range []struct {
		name              string
		inferenceFailures int
		expectedDeadline  time.Time
	}{
		{
			name:             "successful result keeps normal interval",
			expectedDeadline: now.Add(61 * time.Minute),
		},
		{
			name:              "failed result keeps active reservation",
			inferenceFailures: 1,
			expectedDeadline:  reservedUntil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			shared := &fakeMultimodalProbeStateCache{states: map[int64]multimodalProbeState{
				1: {mode: multimodalProbeL7Available, nextInferenceAt: now},
			}}
			scheduler := multimodalProbeScheduler{stateCache: shared}
			_, found, err := scheduler.load(context.Background(), 1)
			require.NoError(t, err)
			require.True(t, found)
			reserved, err := scheduler.reserveInference(context.Background(), 1, now, reservedUntil, 2*time.Hour)
			require.NoError(t, err)
			require.True(t, reserved)
			shared.setErr = errTestSentinel

			err = scheduler.recordInferenceSchedule(
				context.Background(), 1, now.Add(time.Minute), false, test.inferenceFailures, policy, 2*time.Hour,
			)

			require.ErrorIs(t, err, errTestSentinel)
			state, ok := scheduler.snapshot(1)
			require.True(t, ok)
			require.Equal(t, test.expectedDeadline, state.nextInferenceAt)
			require.Equal(t, reservedUntil, shared.states[int64(1)].nextInferenceAt)
		})
	}
}
