package database_test

import (
	"context"
	"database/sql"
	"sync"

	"opencsg.com/csghub-server/builder/store/database"
)

type testRepositoryDeletionJobClient struct {
	mu                 sync.Mutex
	inputs             []database.RepositoryDeletionJobInput
	organizationInputs []database.OrganizationDeletionJobInput
	err                error
	organizationErr    error
}

func (c *testRepositoryDeletionJobClient) InsertOrganizationDeletionJobTx(
	_ context.Context,
	_ *sql.Tx,
	input database.OrganizationDeletionJobInput,
) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.organizationErr != nil {
		return 0, c.organizationErr
	}
	c.organizationInputs = append(c.organizationInputs, input)
	return int64(len(c.organizationInputs)), nil
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

func (c *testRepositoryDeletionJobClient) recordedOrganizationInputs() []database.OrganizationDeletionJobInput {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]database.OrganizationDeletionJobInput(nil), c.organizationInputs...)
}
