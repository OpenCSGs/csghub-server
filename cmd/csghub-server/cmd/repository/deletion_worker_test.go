package repository

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/workhub"
)

type testRepositoryDeletionWorker struct {
	river.WorkerDefaults[workhub.RepositoryDeletionArgs]
}

func (*testRepositoryDeletionWorker) Work(context.Context, *river.Job[workhub.RepositoryDeletionArgs]) error {
	return nil
}

func TestNewRepositoryDeletionRiverConfig(t *testing.T) {
	worker := &testRepositoryDeletionWorker{}
	config := newRepositoryDeletionRiverConfig(worker, 3)

	require.Equal(t, map[string]river.QueueConfig{
		workhub.RepositoryDeletionQueue: {MaxWorkers: 3},
	}, config.Queues)
	require.NotNil(t, config.Workers)
}

func TestNewRepositoryDeletionRiverConfigDefaultsWorkerCount(t *testing.T) {
	config := newRepositoryDeletionRiverConfig(&testRepositoryDeletionWorker{}, 0)

	require.Equal(t, 1, config.Queues[workhub.RepositoryDeletionQueue].MaxWorkers)
}
