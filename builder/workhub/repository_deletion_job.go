package workhub

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

const (
	// RepositoryDeletionQueue is the dedicated queue and job kind for slow
	// repository resource cleanup.
	RepositoryDeletionQueue = "repository_deletion"
	// RepositoryDeletionJobTimeout bounds one Git, LFS, ReBAC, and database
	// cleanup attempt. River retries attempts that return an error.
	RepositoryDeletionJobTimeout = 2 * time.Hour
	// RepositoryDeletionJobMaxAttempts keeps destructive cleanup retryable
	// through extended Git, object-storage, ReBAC, or mirror-service outages.
	RepositoryDeletionJobMaxAttempts = 100
)

// RepositoryDeletionArgs contains the stable cleanup snapshot needed after
// the repository's owning namespace or organization has been deleted.
type RepositoryDeletionArgs struct {
	RepositoryID   int64                  `json:"repository_id"`
	RepositoryType types.RepositoryType   `json:"repository_type"`
	Path           string                 `json:"path"`
	GitalyPath     string                 `json:"gitaly_path"`
	Migrated       bool                   `json:"migrated"`
	OwnerType      database.NamespaceType `json:"owner_type"`
	OwnerUUID      string                 `json:"owner_uuid"`
}

// Kind returns the River kind for repository deletion jobs.
func (RepositoryDeletionArgs) Kind() string {
	return RepositoryDeletionQueue
}

// InsertOpts routes repository deletion jobs to their dedicated queue.
func (RepositoryDeletionArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: RepositoryDeletionQueue}
}

type repositoryDeletionJobClient struct {
	jobClient JobClient
}

// NewRepositoryDeletionJobClient adapts a workhub job client to the database
// repository deletion job interface.
func NewRepositoryDeletionJobClient(jobClient JobClient) database.RepositoryDeletionJobClient {
	return repositoryDeletionJobClient{jobClient: jobClient}
}

// InsertRepositoryDeletionJobTx inserts one repository deletion job inside the
// transaction that soft-deletes the repository.
func (c repositoryDeletionJobClient) InsertRepositoryDeletionJobTx(ctx context.Context, tx *sql.Tx, input database.RepositoryDeletionJobInput) (int64, error) {
	if c.jobClient == nil {
		return 0, fmt.Errorf("workhub job client is required")
	}
	args := RepositoryDeletionArgs{
		RepositoryID:   input.RepositoryID,
		RepositoryType: input.RepositoryType,
		Path:           input.Path,
		GitalyPath:     input.GitalyPath,
		Migrated:       input.Migrated,
		OwnerType:      input.OwnerType,
		OwnerUUID:      input.OwnerUUID,
	}
	return c.jobClient.InsertTx(ctx, tx, args, &InsertOpts{
		Queue:       args.InsertOpts().Queue,
		MaxAttempts: RepositoryDeletionJobMaxAttempts,
	})
}
