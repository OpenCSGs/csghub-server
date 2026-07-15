package workhub

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"opencsg.com/csghub-server/builder/store/database"
)

const (
	// MirrorRepoQueue is the River kind and queue name used for repository mirror jobs.
	MirrorRepoQueue = "mirror_repo"
	// MirrorLFSQueue is the River kind and queue name used for Git LFS mirror jobs.
	MirrorLFSQueue = "mirror_lfs"
	// MirrorRepoJobTimeout is the maximum runtime allowed for one repository mirror job.
	MirrorRepoJobTimeout = 30 * time.Minute
	// MirrorLFSJobTimeout is the maximum runtime allowed for one Git LFS mirror job.
	MirrorLFSJobTimeout = 2 * time.Hour
)

// RepoArgs describes a repository sync task submitted by a workhub caller.
type RepoArgs struct {
	// MirrorID identifies the mirror record to synchronize.
	MirrorID int64 `json:"mirror_id"`
	// RepositoryID identifies the local repository being synchronized.
	RepositoryID int64 `json:"repository_id"`
	// MirrorTaskID identifies the database task that tracks this workhub job.
	MirrorTaskID int64 `json:"mirror_task_id"`
}

// Kind returns the internal task kind for repo sync tasks.
func (RepoArgs) Kind() string {
	return MirrorRepoQueue
}

// InsertOpts returns the default River queue options for repo sync tasks.
func (RepoArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: MirrorRepoQueue,
	}
}

// LFSArgs describes a Git LFS sync task submitted by a workhub caller.
type LFSArgs struct {
	// MirrorID identifies the mirror record to synchronize.
	MirrorID int64 `json:"mirror_id"`
	// RepositoryID identifies the local repository being synchronized.
	RepositoryID int64 `json:"repository_id"`
	// MirrorTaskID identifies the database task that tracks this LFS job.
	MirrorTaskID int64 `json:"mirror_task_id"`
}

// Kind returns the internal task kind for LFS sync tasks.
func (LFSArgs) Kind() string {
	return MirrorLFSQueue
}

// InsertOpts returns the default River queue options for LFS sync tasks.
func (LFSArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: MirrorLFSQueue,
	}
}

// mirrorRepoJobClient adapts the workhub queue client to the database repo job interface.
type mirrorRepoJobClient struct {
	jobClient JobClient
}

// NewMirrorRepoJobClient adapts a workhub job client to the database repo job interface.
func NewMirrorRepoJobClient(jobClient JobClient) database.MirrorJobClient {
	return mirrorRepoJobClient{jobClient: jobClient}
}

// InsertMirrorRepoJobTx inserts one repo workhub job inside the provided transaction.
func (c mirrorRepoJobClient) InsertMirrorRepoJobTx(ctx context.Context, tx *sql.Tx, input database.MirrorJobInput) (int64, error) {
	if c.jobClient == nil {
		return 0, fmt.Errorf("workhub job client is required")
	}
	args := RepoArgs{
		MirrorID:     input.MirrorID,
		RepositoryID: input.RepositoryID,
		MirrorTaskID: input.MirrorTaskID,
	}
	return c.jobClient.InsertTx(ctx, tx, args, &InsertOpts{
		Priority: int(input.Priority),
		Queue:    args.InsertOpts().Queue,
	})
}

// mirrorLFSJobClient adapts the workhub queue client to the database LFS job interface.
type mirrorLFSJobClient struct {
	jobClient JobClient
}

// NewMirrorLFSJobClient adapts a workhub job client to the database LFS job interface.
func NewMirrorLFSJobClient(jobClient JobClient) database.MirrorLFSJobClient {
	return mirrorLFSJobClient{jobClient: jobClient}
}

// InsertMirrorLFSJobTx inserts one LFS workhub job inside the provided transaction.
func (c mirrorLFSJobClient) InsertMirrorLFSJobTx(ctx context.Context, tx *sql.Tx, input database.MirrorLFSJobInput) (int64, error) {
	if c.jobClient == nil {
		return 0, fmt.Errorf("workhub job client is required")
	}
	args := LFSArgs{
		MirrorID:     input.MirrorID,
		RepositoryID: input.RepositoryID,
		MirrorTaskID: input.MirrorTaskID,
	}
	return c.jobClient.InsertTx(ctx, tx, args, &InsertOpts{
		Priority: int(input.Priority),
		Queue:    args.InsertOpts().Queue,
	})
}
