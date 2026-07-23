// Package workhub provides River-backed queue clients for asynchronous workhub
// tasks such as repository mirroring and Git LFS synchronization.
package workhub

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
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

const (
	// defaultJobMaxAttempts allows one initial execution and three retries.
	defaultJobMaxAttempts = 4
	// workClientRetryStep is added for each failed execution.
	workClientRetryStep = 5 * time.Minute
	// workClientRetryJitter spreads retries by ten percent around their base delay.
	workClientRetryJitter = 0.1
)

// JobClient creates and cancels workhub jobs through the application's database
// connection. Use a worker client created by NewWorkClient when a process needs
// to start or stop workers.
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
	// Stop cancels in-progress work, stops the River client, and releases owned database resources.
	Stop(ctx context.Context) error
	// ConfigureUrgentManager creates and binds a process-local urgent manager.
	ConfigureUrgentManager(config UrgentManagerConfig) *UrgentManager
}

// JobArgs is the River job payload contract accepted by workhub queue clients.
type JobArgs = river.JobArgs

// InsertOpts mirrors river.InsertOpts so callers do not need to import River
// when they only enqueue workhub jobs.
type InsertOpts struct {
	// MaxAttempts limits total executions, including the initial attempt.
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
		opts.MaxAttempts = defaultJobMaxAttempts
	}
	// River only accepts priorities from 1 to 4. Legacy mirror rows may still
	// contain the old priority scale, so fall back to a valid normal priority.
	if opts.Priority < 1 || opts.Priority > 4 {
		opts.Priority = 4
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

// linearJitterRetryPolicy schedules retries in five-minute linear steps with ten-percent jitter.
type linearJitterRetryPolicy struct{}

// NextRetry returns the next retry time using River's persisted error count.
func (*linearJitterRetryPolicy) NextRetry(job *rivertype.JobRow) time.Time {
	retry := len(job.Errors) + 1
	baseDelay := time.Duration(retry) * workClientRetryStep
	jitter := rand.Float64()*2*workClientRetryJitter - workClientRetryJitter
	return time.Now().UTC().Add(time.Duration(float64(baseDelay) * (1 + jitter)))
}

// jobClient adapts a database/sql River client to transactional job operations.
// It does not start worker processing.
type jobClient struct {
	client *river.Client[*sql.Tx]
}

// workClient owns a pgx River worker client and its database connection pool. It
// is only intended for worker Start and Stop operations and does not support
// Insert or InsertTx; use NewJobClient when a caller needs to enqueue jobs.
type workClient struct {
	client        *river.Client[pgx.Tx]
	pool          *pgxpool.Pool
	urgentManager *UrgentManager
}

// NewJobClient creates a queue client backed by the application's existing Bun
// database handle. It supports job creation and cancellation; use a work client
// constructor when a process needs to start or stop workers.
func NewJobClient(_ context.Context, db *bun.DB) (JobClient, error) {
	client, err := river.NewClient(riverdatabasesql.New(db.DB), &river.Config{
		SkipUnknownJobCheck: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create workhub client: %w", err)
	}

	return &jobClient{client: client}, nil
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
	config.DiscardedJobRetentionPeriod = -1
	config.RetryPolicy = &linearJitterRetryPolicy{}
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

// RemoveQueue stops one queue producer on this client and waits for claimed work to exit.
func (c *workClient) RemoveQueue(ctx context.Context, queue string) error {
	return c.client.Queues().Remove(ctx, queue)
}

// AddQueue starts one queue producer on this client.
func (c *workClient) AddQueue(queue string, config river.QueueConfig) error {
	return c.client.Queues().Add(queue, config)
}

// ConfigureUrgentManager creates an urgent manager controlled by this River client.
func (c *workClient) ConfigureUrgentManager(config UrgentManagerConfig) *UrgentManager {
	config.QueueController = c
	manager := NewUrgentManager(config)
	c.urgentManager = manager
	return manager
}

// Start starts the River client.
func (c *workClient) Start(ctx context.Context) error {
	if err := c.client.Start(ctx); err != nil {
		return fmt.Errorf("start workhub work client: %w", err)
	}
	return nil
}

// stopWorkClient closes the owned pool only after River has stopped successfully.
func stopWorkClient(ctx context.Context, stopAndCancel func(context.Context) error, closePool func()) error {
	if err := stopAndCancel(ctx); err != nil {
		return fmt.Errorf("stop workhub work client: %w", err)
	}
	closePool()
	return nil
}

// Stop cancels in-progress work, stops the River client, and closes its database pool.
func (c *workClient) Stop(ctx context.Context) error {
	if c.urgentManager != nil {
		c.urgentManager.Close(ErrWorkerShutdown)
	}
	return stopWorkClient(ctx, c.client.StopAndCancel, c.pool.Close)
}
