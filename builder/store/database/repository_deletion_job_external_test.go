package database_test

import (
	"context"
	"database/sql"
	"sync"

	"opencsg.com/csghub-server/builder/store/database"
)

type testRepositoryDeletionJobClient struct {
	mu     sync.Mutex
	inputs []database.RepositoryDeletionJobInput
	err    error
}

func (c *testRepositoryDeletionJobClient) InsertRepositoryDeletionJobTx(
	_ context.Context,
	_ *sql.Tx,
	input database.RepositoryDeletionJobInput,
) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return 0, c.err
	}
	c.inputs = append(c.inputs, input)
	return int64(len(c.inputs)), nil
}

func (c *testRepositoryDeletionJobClient) recordedInputs() []database.RepositoryDeletionJobInput {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]database.RepositoryDeletionJobInput(nil), c.inputs...)
}
