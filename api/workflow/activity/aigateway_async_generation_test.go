package activity

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockaigatewaytask "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/task"
	commontypes "opencsg.com/csghub-server/common/types"
)

func sampleTarget(id int64) commontypes.AIGatewayAsyncGenerationTarget {
	return commontypes.AIGatewayAsyncGenerationTarget{
		ID:                 id,
		ResourceType:       "video",
		ResourceID:         "resource-" + itoa(id),
		ProviderResourceID: "provider-" + itoa(id),
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
		CreatedAt:          time.Now(),
	}
}

func itoa(i int64) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func TestListPendingAIGatewayAsyncGenerationsDelegatesToService(t *testing.T) {
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	want := []commontypes.AIGatewayAsyncGenerationTarget{sampleTarget(1), sampleTarget(2)}
	mockService.EXPECT().ListPendingGenerations(mock.Anything).Return(want, nil).Once()

	activities := &Activities{asyncGenerationService: mockService}

	got, err := activities.ListPendingAIGatewayAsyncGenerations(context.Background())

	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestListPendingAIGatewayAsyncGenerationsPropagatesServiceError(t *testing.T) {
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	expectedErr := errors.New("db unavailable")
	mockService.EXPECT().ListPendingGenerations(mock.Anything).Return(nil, expectedErr).Once()

	activities := &Activities{asyncGenerationService: mockService}

	got, err := activities.ListPendingAIGatewayAsyncGenerations(context.Background())

	require.Nil(t, got)
	require.ErrorIs(t, err, expectedErr)
}

func TestInspectAndMeterAIGatewayAsyncGenerationsEmptyReturnsNil(t *testing.T) {
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	activities := &Activities{asyncGenerationService: mockService}

	err := activities.InspectAndMeterAIGatewayAsyncGenerations(context.Background(), nil)
	require.NoError(t, err)

	err = activities.InspectAndMeterAIGatewayAsyncGenerations(context.Background(), []commontypes.AIGatewayAsyncGenerationTarget{})
	require.NoError(t, err)

	// No expectations on the mock → verifies the service was never called.
}

func TestInspectAndMeterAIGatewayAsyncGenerationsInvokesServiceForEachTarget(t *testing.T) {
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	targets := []commontypes.AIGatewayAsyncGenerationTarget{
		sampleTarget(1),
		sampleTarget(2),
		sampleTarget(3),
	}

	for _, target := range targets {
		mockService.EXPECT().InspectAndMeter(mock.Anything, target).Return(nil).Once()
	}

	activities := &Activities{asyncGenerationService: mockService}

	err := activities.InspectAndMeterAIGatewayAsyncGenerations(context.Background(), targets)

	require.NoError(t, err)
}

func TestInspectAndMeterAIGatewayAsyncGenerationsSwallowsServiceErrors(t *testing.T) {
	// Documents the current behavior: the activity logs and returns nil even when
	// every InspectAndMeter call fails. Callers that want failure visibility must
	// change the function to return the errgroup error.
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	targets := []commontypes.AIGatewayAsyncGenerationTarget{
		sampleTarget(10),
		sampleTarget(11),
		sampleTarget(12),
	}
	for _, target := range targets {
		mockService.EXPECT().InspectAndMeter(mock.Anything, target).
			Return(errors.New("processor not found")).Once()
	}

	activities := &Activities{asyncGenerationService: mockService}

	err := activities.InspectAndMeterAIGatewayAsyncGenerations(context.Background(), targets)

	require.NoError(t, err, "all service errors must be swallowed")
}

func TestInspectAndMeterAIGatewayAsyncGenerationsMixedSuccessAndFailure(t *testing.T) {
	// Even when some goroutines succeed and others fail, the function still returns nil.
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	failing := sampleTarget(20)
	succeeding := sampleTarget(21)
	mockService.EXPECT().InspectAndMeter(mock.Anything, failing).
		Return(errors.New("transient redis error")).Once()
	mockService.EXPECT().InspectAndMeter(mock.Anything, succeeding).Return(nil).Once()

	activities := &Activities{asyncGenerationService: mockService}

	err := activities.InspectAndMeterAIGatewayAsyncGenerations(
		context.Background(),
		[]commontypes.AIGatewayAsyncGenerationTarget{failing, succeeding},
	)

	require.NoError(t, err)
}

func TestInspectAndMeterAIGatewayAsyncGenerationsRespectsCanceledContext(t *testing.T) {
	// When the parent context is already canceled, the errgroup-derived ctx is
	// also canceled, so goroutines short-circuit and the service is not called.
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	targets := []commontypes.AIGatewayAsyncGenerationTarget{
		sampleTarget(30),
		sampleTarget(31),
	}
	// No EXPECT() calls on the mock → verifies InspectAndMeter is never reached.

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	activities := &Activities{asyncGenerationService: mockService}

	err := activities.InspectAndMeterAIGatewayAsyncGenerations(ctx, targets)
	require.NoError(t, err)
}

func TestInspectAndMeterAIGatewayAsyncGenerationsCancellationMidFlightStopsWork(t *testing.T) {
	// Cancel the parent context while the service is blocked, then verify that
	// the function returns without waiting for blocked goroutines to complete.
	// This is the production case: a Temporal activity times out and the
	// derived ctx is canceled.
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	target := sampleTarget(40)

	started := make(chan struct{})
	release := make(chan struct{})
	mockService.EXPECT().InspectAndMeter(mock.Anything, target).
		Run(func(ctx context.Context, _ commontypes.AIGatewayAsyncGenerationTarget) {
			close(started)
			<-ctx.Done()
		}).
		Return(context.Canceled).Once()

	ctx, cancel := context.WithCancel(context.Background())
	activities := &Activities{asyncGenerationService: mockService}

	done := make(chan error, 1)
	go func() {
		done <- activities.InspectAndMeterAIGatewayAsyncGenerations(ctx, []commontypes.AIGatewayAsyncGenerationTarget{target})
	}()

	<-started
	cancel()
	close(release)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("function did not return after parent context was canceled")
	}
}

func TestInspectAndMeterAIGatewayAsyncGenerationsProcessesAllTargetsConcurrently(t *testing.T) {
	// With more targets than the errgroup limit (20), confirm that the function
	// still processes every target. Concurrency timing is intentionally loose:
	// we only assert the *existence* of overlap, not exact ordering.
	const targetCount = 50
	mockService := mockaigatewaytask.NewMockAsyncGenerationService(t)
	targets := make([]commontypes.AIGatewayAsyncGenerationTarget, targetCount)
	for i := range targetCount {
		targets[i] = sampleTarget(int64(100 + i))
	}

	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	seen := sync.Map{}

	for _, target := range targets {
		mockService.EXPECT().InspectAndMeter(mock.Anything, target).
			Run(func(_ context.Context, _ commontypes.AIGatewayAsyncGenerationTarget) {
				cur := inFlight.Add(1)
				for {
					prev := maxInFlight.Load()
					if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
						break
					}
				}
				// Hold briefly so concurrent goroutines overlap.
				time.Sleep(2 * time.Millisecond)
				inFlight.Add(-1)
			}).
			Return(nil).Once()
		seen.Store(target.ID, struct{}{})
	}

	activities := &Activities{asyncGenerationService: mockService}

	err := activities.InspectAndMeterAIGatewayAsyncGenerations(context.Background(), targets)
	require.NoError(t, err)

	require.Greater(t, maxInFlight.Load(), int32(1),
		"expected multiple goroutines to run concurrently, but max in-flight was %d", maxInFlight.Load())
	require.LessOrEqual(t, maxInFlight.Load(), int32(aigatewayAsyncGenerationConcurrency),
		"max in-flight must not exceed the errgroup limit")

	for _, target := range targets {
		_, ok := seen.Load(target.ID)
		require.True(t, ok, "target %d was never processed", target.ID)
	}
}
