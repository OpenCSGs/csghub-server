package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/logcollector/component"
	ltypes "opencsg.com/csghub-server/logcollector/types"
)

func Test_logFactory_Start(t *testing.T) {
	// Create a mock LogCollectorManager
	mockWorker1 := new(mockcomponent.MockLogCollectorManager)
	mockWorker1.On("Start", mock.Anything).Return(nil)

	mockWorker2 := new(mockcomponent.MockLogCollectorManager)
	mockWorker2.On("Start", mock.Anything).Return(nil)

	// Create a logFactory with the mock workers
	lf := &logFactory{
		workers: map[string]LogCollectorManager{
			"cluster1": mockWorker1,
			"cluster2": mockWorker2,
		},
	}

	// Call the Start method
	err := lf.Start()

	// Assert that the error is nil
	assert.NoError(t, err)

	// Assert that the Start method was called on each worker
	mockWorker1.AssertCalled(t, "Start", mock.Anything)
	mockWorker2.AssertCalled(t, "Start", mock.Anything)
}

func Test_logFactory_Start_Error(t *testing.T) {
	// Create a mock LogCollectorManager that returns an error
	mockWorker1 := new(mockcomponent.MockLogCollectorManager)
	expectedError := errors.New("worker start error")
	mockWorker1.On("Start", mock.Anything).Return(expectedError)

	// Create a logFactory with the mock worker
	lf := &logFactory{
		workers: map[string]LogCollectorManager{
			"cluster1": mockWorker1,
		},
	}

	// Call the Start method
	err := lf.Start()

	// Assert that the error is the expected error
	assert.Equal(t, expectedError, err)

	// Assert that the Start method was called on the worker
	mockWorker1.AssertCalled(t, "Start", mock.Anything)
}

func Test_logFactory_Stop(t *testing.T) {
	// Create mock LogCollectorManager instances
	mockWorker1 := new(mockcomponent.MockLogCollectorManager)
	mockWorker1.On("Stop").Return(nil)

	mockWorker2 := new(mockcomponent.MockLogCollectorManager)
	mockWorker2.On("Stop").Return(nil)

	// Create a logFactory with the mock workers
	ctx, cancel := context.WithCancel(context.Background())
	lf := &logFactory{
		ctx:    ctx,
		cancel: cancel,
		workers: map[string]LogCollectorManager{
			"cluster1": mockWorker1,
			"cluster2": mockWorker2,
		},
	}

	// Call the Stop method
	lf.Stop()

	// Assert that the Stop method was called on each worker
	mockWorker1.AssertCalled(t, "Stop")
	mockWorker2.AssertCalled(t, "Stop")

	// Assert that the context is canceled
	<-ctx.Done()
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
}

func Test_logFactory_GetStats(t *testing.T) {
	// Create mock LogCollectorManager instances
	mockWorker1 := new(mockcomponent.MockLogCollectorManager)
	stats1 := &ltypes.CollectorStats{TotalLogsCollected: 100}
	mockWorker1.On("GetStats").Return(stats1)

	mockWorker2 := new(mockcomponent.MockLogCollectorManager)
	stats2 := &ltypes.CollectorStats{TotalLogsCollected: 200}
	mockWorker2.On("GetStats").Return(stats2)

	// Create a logFactory with the mock workers
	lf := &logFactory{
		workers: map[string]LogCollectorManager{
			"cluster1": mockWorker1,
			"cluster2": mockWorker2,
		},
	}

	// Call the GetStats method
	allStats := lf.GetStats()

	// Assert that the stats are aggregated correctly
	assert.Len(t, allStats, 2)
	assert.Equal(t, stats1, allStats["cluster1"])
	assert.Equal(t, stats2, allStats["cluster2"])

	// Assert that GetStats was called on each worker
	mockWorker1.AssertCalled(t, "GetStats")
	mockWorker2.AssertCalled(t, "GetStats")
}
