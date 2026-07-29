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
	// MirrorRepoUrgentQueue is the River queue used for urgent repository mirror jobs.
	MirrorRepoUrgentQueue = "mirror_repo_urgent"
	// MirrorLFSQueue is the River kind and queue name used for Git LFS mirror jobs.
	MirrorLFSQueue = "mirror_lfs"
	// MirrorLFSUrgentQueue is the River queue used for urgent Git LFS mirror jobs.
	MirrorLFSUrgentQueue = "mirror_lfs_urgent"
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
	// Urgent routes this job through the urgent repository queue and execution mode.
	Urgent bool `json:"urgent"`
}

// Kind returns the internal task kind for repo sync tasks.
func (RepoArgs) Kind() string {
	return MirrorRepoQueue
}

// InsertOpts returns the default River queue options for repo sync tasks.
func (args RepoArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: RepoQueue(args.Urgent),
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
	// Urgent routes this job through the urgent LFS queue and execution mode.
	Urgent bool `json:"urgent"`
}

// Kind returns the internal task kind for LFS sync tasks.
func (LFSArgs) Kind() string {
	return MirrorLFSQueue
}

// InsertOpts returns the default River queue options for LFS sync tasks.
func (args LFSArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: LFSQueue(args.Urgent),
	}
}

// RepoQueue returns the repository queue for the requested execution mode.
func RepoQueue(urgent bool) string {
	if urgent {
		return MirrorRepoUrgentQueue
	}
	return MirrorRepoQueue
}

// LFSQueue returns the Git LFS queue for the requested execution mode.
func LFSQueue(urgent bool) string {
	if urgent {
		return MirrorLFSUrgentQueue
	}
	return MirrorLFSQueue
}

// ValidateRepoQueue verifies that a repository job was claimed from its selected queue.
func ValidateRepoQueue(args RepoArgs, actualQueue string) error {
	expectedQueue := RepoQueue(args.Urgent)
	if actualQueue != expectedQueue {
		return fmt.Errorf("mirror repo queue mismatch: urgent=%t expected=%s actual=%s", args.Urgent, expectedQueue, actualQueue)
	}
	return nil
}

// ValidateLFSQueue verifies that an LFS job was claimed from its selected queue.
func ValidateLFSQueue(args LFSArgs, actualQueue string) error {
	expectedQueue := LFSQueue(args.Urgent)
	if actualQueue != expectedQueue {
		return fmt.Errorf("mirror LFS queue mismatch: urgent=%t expected=%s actual=%s", args.Urgent, expectedQueue, actualQueue)
	}
	return nil
}

// UrgentMaxWorkers returns the urgent queue concurrency derived from normal concurrency.
func UrgentMaxWorkers(normalMaxWorkers int) int {
	urgentMaxWorkers := normalMaxWorkers / 2
	if urgentMaxWorkers < 1 {
		return 1
	}
	return urgentMaxWorkers
}

// MirrorJobClientConfig controls River attempts for mirror job adapters.
type MirrorJobClientConfig struct {
	// MaxRetryCount is the number of retries allowed after the initial execution.
	MaxRetryCount int
}

// mirrorMaxAttempts converts configured retries to River's total execution limit.
func mirrorMaxAttempts(maxRetryCount int) int {
	if maxRetryCount < 0 {
		maxRetryCount = 0
	}
	return maxRetryCount + 1
}

// mirrorRepoJobClient adapts the workhub queue client to the database repo job interface.
type mirrorRepoJobClient struct {
	jobClient JobClient
	config    MirrorJobClientConfig
}

// NewMirrorRepoJobClient adapts a workhub job client to the database repo job interface.
func NewMirrorRepoJobClient(jobClient JobClient, config MirrorJobClientConfig) database.MirrorJobClient {
	return mirrorRepoJobClient{jobClient: jobClient, config: config}
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
		Urgent:       input.Urgent,
	}
	return c.jobClient.InsertTx(ctx, tx, args, &InsertOpts{
		MaxAttempts: mirrorMaxAttempts(c.config.MaxRetryCount),
		Priority:    int(input.Priority),
		Queue:       args.InsertOpts().Queue,
	})
}

// mirrorLFSJobClient adapts the workhub queue client to the database LFS job interface.
type mirrorLFSJobClient struct {
	jobClient JobClient
	config    MirrorJobClientConfig
}

// NewMirrorLFSJobClient adapts a workhub job client to the database LFS job interface.
func NewMirrorLFSJobClient(jobClient JobClient, config MirrorJobClientConfig) database.MirrorLFSJobClient {
	return mirrorLFSJobClient{jobClient: jobClient, config: config}
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
		Urgent:       input.Urgent,
	}
	return c.jobClient.InsertTx(ctx, tx, args, &InsertOpts{
		MaxAttempts: mirrorMaxAttempts(c.config.MaxRetryCount),
		Priority:    int(input.Priority),
		Queue:       args.InsertOpts().Queue,
	})
}
