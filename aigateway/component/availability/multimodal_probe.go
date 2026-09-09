package availability

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type multimodalProbeMode uint8

const (
	multimodalProbeL7Available multimodalProbeMode = iota
	multimodalProbeInferenceOnly
)

func (m multimodalProbeMode) String() string {
	switch m {
	case multimodalProbeL7Available:
		return "l7_available"
	case multimodalProbeInferenceOnly:
		return "inference_only"
	default:
		return "unknown"
	}
}

func parseMultimodalProbeMode(value string) (multimodalProbeMode, error) {
	switch value {
	case "l7_available":
		return multimodalProbeL7Available, nil
	case "inference_only":
		return multimodalProbeInferenceOnly, nil
	default:
		return multimodalProbeL7Available, fmt.Errorf("invalid multimodal probe mode %q", value)
	}
}

type multimodalProbePolicy struct {
	retryInterval     time.Duration
	inferenceInterval time.Duration
	failureThreshold  int
}

type multimodalProbeState struct {
	mode            multimodalProbeMode
	nextInferenceAt time.Time
}

type multimodalProbePlan struct {
	skipAll      bool
	inferenceDue bool
}

type multimodalProbeScheduler struct {
	mu         sync.Mutex
	states     map[int64]multimodalProbeState
	stateCache multimodalProbeStateCache
}

func newMultimodalProbeScheduler(stateCache StateCache) multimodalProbeScheduler {
	probeStateCache, _ := stateCache.(multimodalProbeStateCache)
	return multimodalProbeScheduler{stateCache: probeStateCache}
}

func (s multimodalProbeState) plan(now time.Time) multimodalProbePlan {
	inferenceDue := !now.Before(s.nextInferenceAt)
	return multimodalProbePlan{
		skipAll:      s.mode == multimodalProbeInferenceOnly && !inferenceDue,
		inferenceDue: inferenceDue,
	}
}

func (s *multimodalProbeScheduler) plan(upstreamID int64, now time.Time) multimodalProbePlan {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.states[upstreamID].plan(now)
}

func (s *multimodalProbeScheduler) load(ctx context.Context, upstreamID int64) (multimodalProbeState, bool, error) {
	s.mu.Lock()
	state, ok := s.states[upstreamID]
	s.mu.Unlock()
	if ok {
		return state, true, nil
	}
	if s.stateCache == nil || !s.stateCache.Enabled() {
		return multimodalProbeState{}, false, nil
	}

	state, err := s.stateCache.GetMultimodalProbeState(ctx, upstreamID)
	if errors.Is(err, errStateCacheMiss) {
		return multimodalProbeState{}, false, nil
	}
	if err != nil {
		return multimodalProbeState{}, false, err
	}
	s.setLocal(upstreamID, state)
	return state, true, nil
}

func (s *multimodalProbeScheduler) store(ctx context.Context, upstreamID int64, state multimodalProbeState, ttl time.Duration) error {
	if s.stateCache != nil && s.stateCache.Enabled() {
		if err := s.stateCache.SetMultimodalProbeState(ctx, upstreamID, state, ttl); err != nil {
			return err
		}
	}
	s.setLocal(upstreamID, state)
	return nil
}

func (s *multimodalProbeScheduler) reserveInference(
	ctx context.Context,
	upstreamID int64,
	now time.Time,
	reservedUntil time.Time,
	ttl time.Duration,
) (bool, error) {
	s.mu.Lock()
	state := s.states[upstreamID]
	if s.stateCache == nil || !s.stateCache.Enabled() {
		if now.Before(state.nextInferenceAt) {
			s.mu.Unlock()
			return false, nil
		}
		state.nextInferenceAt = reservedUntil
		if s.states == nil {
			s.states = make(map[int64]multimodalProbeState)
		}
		s.states[upstreamID] = state
		s.mu.Unlock()
		return true, nil
	}
	s.mu.Unlock()

	reservedState, reserved, err := s.stateCache.TryReserveMultimodalInference(
		ctx,
		upstreamID,
		state,
		now,
		reservedUntil,
		ttl,
	)
	if err != nil {
		return false, err
	}
	s.setLocal(upstreamID, reservedState)
	return reserved, nil
}

func (s *multimodalProbeScheduler) recordL7Available(
	ctx context.Context,
	upstreamID int64,
	ttl time.Duration,
) error {
	s.mu.Lock()
	state, ok := s.states[upstreamID]
	s.mu.Unlock()

	if !ok {
		return nil
	}
	if state.mode == multimodalProbeL7Available {
		return nil
	}
	state.mode = multimodalProbeL7Available
	if err := s.store(ctx, upstreamID, state, ttl); err != nil {
		s.setLocal(upstreamID, state)
		return err
	}
	return nil
}

func (s *multimodalProbeScheduler) recordInferenceSchedule(
	ctx context.Context,
	upstreamID int64,
	now time.Time,
	usedFallback bool,
	inferenceFailures int,
	policy multimodalProbePolicy,
	ttl time.Duration,
) error {
	s.mu.Lock()
	state := s.states[upstreamID]
	s.mu.Unlock()
	reservedUntil := state.nextInferenceAt
	if usedFallback {
		state.mode = multimodalProbeInferenceOnly
	} else {
		state.mode = multimodalProbeL7Available
	}
	state.nextInferenceAt = nextMultimodalInferenceAt(now, inferenceFailures, policy)
	if err := s.store(ctx, upstreamID, state, ttl); err != nil {
		if state.nextInferenceAt.Before(reservedUntil) {
			state.nextInferenceAt = reservedUntil
		}
		s.setLocal(upstreamID, state)
		return err
	}
	return nil
}

func nextMultimodalInferenceAt(now time.Time, inferenceFailures int, policy multimodalProbePolicy) time.Time {
	if inferenceFailures > 0 && inferenceFailures < policy.failureThreshold {
		return now.Add(policy.retryInterval)
	}
	return now.Add(policy.inferenceInterval)
}

func (s *multimodalProbeScheduler) recordInferenceOnly(ctx context.Context, upstreamID int64, ttl time.Duration) error {
	s.mu.Lock()
	state := s.states[upstreamID]
	s.mu.Unlock()
	state.mode = multimodalProbeInferenceOnly
	if err := s.store(ctx, upstreamID, state, ttl); err != nil {
		s.setLocal(upstreamID, state)
		return err
	}
	return nil
}

func (s *multimodalProbeScheduler) cleanup(activeUpstreamIDs []int64) {
	active := make(map[int64]struct{}, len(activeUpstreamIDs))
	for _, upstreamID := range activeUpstreamIDs {
		active[upstreamID] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for upstreamID := range s.states {
		if _, ok := active[upstreamID]; !ok {
			delete(s.states, upstreamID)
		}
	}
}

func (s *multimodalProbeScheduler) deleteIfCached(ctx context.Context, upstreamID int64) error {
	s.mu.Lock()
	_, ok := s.states[upstreamID]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	if s.stateCache != nil && s.stateCache.Enabled() {
		if err := s.stateCache.DeleteMultimodalProbeState(ctx, upstreamID); err != nil {
			return err
		}
	}
	s.mu.Lock()
	delete(s.states, upstreamID)
	s.mu.Unlock()
	return nil
}

func (s *multimodalProbeScheduler) reset() {
	s.mu.Lock()
	s.states = nil
	s.mu.Unlock()
}

func (s *multimodalProbeScheduler) setLocal(upstreamID int64, state multimodalProbeState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.states == nil {
		s.states = make(map[int64]multimodalProbeState)
	}
	s.states[upstreamID] = state
}

func (s *multimodalProbeScheduler) snapshot(upstreamID int64) (multimodalProbeState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[upstreamID]
	return state, ok
}
