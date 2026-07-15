// Package workhub provides River-backed queue clients for asynchronous workhub
// tasks such as repository mirroring and Git LFS synchronization.
package workhub

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
	"github.com/uptrace/bun"
)

// workClientRescueStuckJobsAfter is the minimum running duration before River
// considers a job eligible for stuck-job rescue.
const workClientRescueStuckJobsAfter = MirrorLFSJobTimeout + (5 * time.Minute)

// JobClient enqueues workhub jobs from services that already own their database
// connection. It is intended for transactional job creation and cancellation;
// use a worker client created by NewWorkClient when a process needs to start or
// stop workers.
type JobClient interface {
	// Insert enqueues a job outside an existing database transaction.
	Insert(ctx context.Context, args JobArgs, opts *InsertOpts) (int64, error)
	// InsertTx enqueues a job atomically inside an existing database transaction.
	InsertTx(ctx context.Context, tx *sql.Tx, args JobArgs, opts *InsertOpts) (int64, error)
	// JobCancelTx cancels a job atomically inside an existing database transaction.
	JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error
}

// WorkClient controls the lifecycle of a River worker process. It is only
// intended for worker Start and Stop operations; use NewJobClient when a caller
// needs to enqueue jobs with Insert or InsertTx.
type WorkClient interface {
	// Start begins polling queues and executing registered workers.
	Start(ctx context.Context) error
	// Stop drains the River client and releases owned database resources.
	Stop(ctx context.Context) error
}

// JobArgs is the River job payload contract accepted by workhub queue clients.
type JobArgs = river.JobArgs

// InsertOpts mirrors river.InsertOpts so callers do not need to import River
// when they only enqueue workhub jobs.
type InsertOpts struct {
	// MaxAttempts limits how many times River retries a failed job.
	MaxAttempts int
	// Metadata stores caller-defined JSON metadata alongside the job.
	Metadata []byte
	// Pending keeps the job out of the available queue until it is explicitly
	// made available.
	Pending bool
	// Priority controls execution order within a queue; River treats 1 as
	// highest and 4 as lowest.
	Priority int
	// Queue selects the River queue that should receive the job.
	Queue string
	// ScheduledAt delays job execution until the specified time.
	ScheduledAt time.Time
	// Tags stores searchable labels for job filtering and observability.
	Tags []string
}

// riverInsertOpts converts local enqueue options to River's native type.
func (opts *InsertOpts) riverInsertOpts() *river.InsertOpts {
	if opts == nil {
		return nil
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 12
	}
	// River only accepts priorities from 1 to 4. Legacy mirror rows may still
	// contain the old priority scale, so fall back to a valid normal priority.
	if opts.Priority < 1 || opts.Priority > 4 {
		opts.Priority = 3
	}
	return &river.InsertOpts{
		MaxAttempts: opts.MaxAttempts,
		Metadata:    opts.Metadata,
		Pending:     opts.Pending,
		Priority:    opts.Priority,
		Queue:       opts.Queue,
		ScheduledAt: opts.ScheduledAt,
		Tags:        opts.Tags,
	}
}

// jobClient adapts a database/sql River client to the workhub JobClient API. It
// is only intended for transactional job operations and does not start worker
// processing; use a work client when a process needs to run workers.
type jobClient struct {
	client *river.Client[*sql.Tx]
}

// workClient owns a pgx River worker client and its database connection pool. It
// is only intended for worker Start and Stop operations and does not support
// Insert or InsertTx; use NewJobClient when a caller needs to enqueue jobs.
type workClient struct {
	client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
}

// NewJobClient creates a queue client backed by the application's existing Bun
// database handle. The returned client is only intended for job creation and
// cancellation; use a work client constructor when a process needs to start or
// stop workers.
func NewJobClient(_ context.Context, db *bun.DB) (JobClient, error) {
	client, err := river.NewClient(riverdatabasesql.New(db.DB), &river.Config{
		SkipUnknownJobCheck: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create workhub client: %w", err)
	}

	return &jobClient{
		client: client,
	}, nil
}

// NewWorkClient creates a worker client for a specific River configuration and
// owns the pgx pool used by that worker process. The returned client is only
// intended for worker Start and Stop operations and does not support Insert or
// InsertTx; use NewJobClient when a caller needs to enqueue jobs.
func NewWorkClient(ctx context.Context, dsn string, config *river.Config) (WorkClient, error) {
	dbPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create workhub database pool: %w", err)
	}

	config.SkipUnknownJobCheck = true
	config.RescueStuckJobsAfter = workClientRescueStuckJobsAfter
	client, err := river.NewClient(riverpgxv5.New(dbPool), config)
	if err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("create workhub client: %w", err)
	}

	return &workClient{
		client: client,
		pool:   dbPool,
	}, nil
}

// Insert inserts a task with River.
func (c *jobClient) Insert(ctx context.Context, args JobArgs, opts *InsertOpts) (int64, error) {
	result, err := c.client.Insert(ctx, args, opts.riverInsertOpts())
	if err != nil {
		return 0, fmt.Errorf("insert workhub job: %w", err)
	}
	return result.Job.ID, nil
}

// InsertTx inserts a task with River in an existing database/sql transaction.
func (c *jobClient) InsertTx(ctx context.Context, tx *sql.Tx, args JobArgs, opts *InsertOpts) (int64, error) {
	result, err := c.client.InsertTx(ctx, tx, args, opts.riverInsertOpts())
	if err != nil {
		return 0, fmt.Errorf("insert workhub job in transaction: %w", err)
	}
	return result.Job.ID, nil
}

// JobCancelTx cancels a job with River in an existing database/sql transaction.
func (c *jobClient) JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error {
	if _, err := c.client.JobCancelTx(ctx, tx, jobID); err != nil {
		if errors.Is(err, rivertype.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("cancel workhub job in transaction: %w", err)
	}
	return nil
}

// Start starts the River client.
func (c *workClient) Start(ctx context.Context) error {
	if err := c.client.Start(ctx); err != nil {
		return fmt.Errorf("start workhub work client: %w", err)
	}
	return nil
}

// Stop stops the River client and closes its database pool.
func (c *workClient) Stop(ctx context.Context) error {
	if err := c.client.Stop(ctx); err != nil {
		return fmt.Errorf("stop workhub work client: %w", err)
	}
	c.pool.Close()
	return nil
}
