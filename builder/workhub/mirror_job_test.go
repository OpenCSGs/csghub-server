package workhub

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// fakeJobClient records workhub enqueue arguments for mirror adapter tests.
type fakeJobClient struct {
	tx     *sql.Tx
	args   JobArgs
	opts   *InsertOpts
	called bool
}

// Insert records non-transactional enqueue arguments.
func (c *fakeJobClient) Insert(ctx context.Context, args JobArgs, opts *InsertOpts) (int64, error) {
	c.args = args
	c.opts = opts
	c.called = true
	return 123, nil
}

// InsertTx records transactional enqueue arguments.
func (c *fakeJobClient) InsertTx(ctx context.Context, tx *sql.Tx, args JobArgs, opts *InsertOpts) (int64, error) {
	c.tx = tx
	c.args = args
	c.opts = opts
	c.called = true
	return 123, nil
}

// JobCancelTx records no data because mirror adapter tests only enqueue jobs.
func (c *fakeJobClient) JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error {
	return nil
}

// TestMirrorJobArgsJSONOnlyCarriesStableIDs verifies River payloads keep only stable database identifiers.
func TestMirrorJobArgsJSONOnlyCarriesStableIDs(t *testing.T) {
	repoPayload, err := json.Marshal(RepoArgs{
		MirrorID:     7,
		RepositoryID: 11,
		MirrorTaskID: 3,
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"mirror_id":7,"repository_id":11,"mirror_task_id":3}`, string(repoPayload))

	lfsPayload, err := json.Marshal(LFSArgs{
		MirrorID:     7,
		RepositoryID: 11,
		MirrorTaskID: 3,
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"mirror_id":7,"repository_id":11,"mirror_task_id":3}`, string(lfsPayload))
}

// TestMirrorJobArgsUseMirrorQueues verifies mirror jobs are routed to stable River queues.
func TestMirrorJobArgsUseMirrorQueues(t *testing.T) {
	require.Equal(t, MirrorRepoQueue, RepoArgs{}.Kind())
	require.Equal(t, MirrorRepoQueue, RepoArgs{}.InsertOpts().Queue)
	require.Equal(t, MirrorLFSQueue, LFSArgs{}.Kind())
	require.Equal(t, MirrorLFSQueue, LFSArgs{}.InsertOpts().Queue)
}

// TestInsertOptsRiverInsertOptsDefaultsInvalidPriority verifies legacy priorities cannot break River inserts.
func TestInsertOptsRiverInsertOptsDefaultsInvalidPriority(t *testing.T) {
	require.Equal(t, 3, (&InsertOpts{Priority: 12}).riverInsertOpts().Priority)
	require.Equal(t, 3, (&InsertOpts{Priority: 0}).riverInsertOpts().Priority)
	require.Equal(t, 4, (&InsertOpts{Priority: 4}).riverInsertOpts().Priority)
}

// TestMirrorRepoJobClientInsertMirrorRepoJobTx verifies repo jobs are mapped to workhub payloads.
func TestMirrorRepoJobClientInsertMirrorRepoJobTx(t *testing.T) {
	ctx := context.TODO()
	jobClient := &fakeJobClient{}
	client := NewMirrorRepoJobClient(jobClient)

	jobID, err := client.InsertMirrorRepoJobTx(ctx, nil, database.MirrorJobInput{
		MirrorID:     42,
		RepositoryID: 11,
		MirrorTaskID: 7,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	require.Equal(t, int64(123), jobID)
	require.True(t, jobClient.called)
	require.IsType(t, RepoArgs{}, jobClient.args)
	require.Equal(t, RepoArgs{
		MirrorID:     42,
		RepositoryID: 11,
		MirrorTaskID: 7,
	}, jobClient.args)
	require.NotNil(t, jobClient.opts)
	require.Equal(t, int(types.ASAPMirrorPriority), jobClient.opts.Priority)
	require.Equal(t, MirrorRepoQueue, jobClient.opts.Queue)
	require.True(t, jobClient.opts.ScheduledAt.IsZero())
}

// TestMirrorLFSJobClientInsertMirrorLFSJobTx verifies LFS jobs are mapped to workhub payloads.
func TestMirrorLFSJobClientInsertMirrorLFSJobTx(t *testing.T) {
	ctx := context.TODO()
	jobClient := &fakeJobClient{}
	client := NewMirrorLFSJobClient(jobClient)

	jobID, err := client.InsertMirrorLFSJobTx(ctx, nil, database.MirrorLFSJobInput{
		MirrorID:     42,
		RepositoryID: 11,
		MirrorTaskID: 7,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	require.Equal(t, int64(123), jobID)
	require.True(t, jobClient.called)
	require.IsType(t, LFSArgs{}, jobClient.args)
	require.Equal(t, LFSArgs{
		MirrorID:     42,
		RepositoryID: 11,
		MirrorTaskID: 7,
	}, jobClient.args)
	require.NotNil(t, jobClient.opts)
	require.Equal(t, int(types.ASAPMirrorPriority), jobClient.opts.Priority)
	require.Equal(t, MirrorLFSQueue, jobClient.opts.Queue)
	require.True(t, jobClient.opts.ScheduledAt.IsZero())
}
