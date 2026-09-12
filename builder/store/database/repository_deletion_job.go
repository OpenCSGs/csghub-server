package database

import (
	"context"
	"database/sql"

	"opencsg.com/csghub-server/common/types"
)

// RepositoryDeletionJobClient inserts repository deletion jobs in the same
// database transaction that makes the repository unavailable.
type RepositoryDeletionJobClient interface {
	InsertRepositoryDeletionJobTx(ctx context.Context, tx *sql.Tx, input RepositoryDeletionJobInput) (int64, error)
}

// RepositoryDeletionJobInput is the stable repository cleanup snapshot passed
// to the asynchronous deletion worker.
type RepositoryDeletionJobInput struct {
	RepositoryID   int64
	RepositoryType types.RepositoryType
	Path           string
	GitalyPath     string
	Migrated       bool
	OwnerType      NamespaceType
	OwnerUUID      string
}
