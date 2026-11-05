package component

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// RebacTupleWorker applies permission events without producing child jobs.
// River serializes permission groups; retries wait and terminal failures release successors.
type RebacTupleWorker struct {
	river.WorkerDefaults[workhub.RebacTupleArgs]
	authorizer rebac.Authorizer
}

// NewRebacTupleWorker uses the supplied authorizer for every permission event.
func NewRebacTupleWorker(authorizer rebac.Authorizer) *RebacTupleWorker {
	return &RebacTupleWorker{authorizer: authorizer}
}

// Work applies the complete event and propagates failures to River for retry.
func (w *RebacTupleWorker) Work(ctx context.Context, job *river.Job[workhub.RebacTupleArgs]) (err error) {
	if job == nil {
		return fmt.Errorf("tuple job is required")
	}
	started := time.Now()
	count := 0
	for _, operation := range job.Args.Operations {
		count += len(operation.Relationships)
	}
	attrs := []any{"source", job.Args.Source, "model_id", job.Args.ModelID, "relationship_count", count}
	if job.JobRow != nil {
		attrs = append(attrs, "job_id", job.ID, "attempt", job.Attempt)
	}
	defer func() {
		attrs = append(attrs, "duration", time.Since(started))
		if err != nil {
			slog.ErrorContext(ctx, "tuple mutation attempt failed", append(attrs, "error", err)...)
		} else {
			slog.DebugContext(ctx, "tuple mutation attempt completed", attrs...)
		}
	}()
	if err := workhub.ValidateRebacTupleJob(job.Args); err != nil {
		return err
	}
	return applyRebacTuple(ctx, w.authorizer, job.Args.RebacTuple)
}

// Timeout bounds tuple execution independently of long-running mirror jobs.
func (*RebacTupleWorker) Timeout(*river.Job[workhub.RebacTupleArgs]) time.Duration {
	return workhub.RebacTupleJobTimeout
}

// NextRetry uses River's default policy without inheriting the mirror retry interval.
func (*RebacTupleWorker) NextRetry(job *river.Job[workhub.RebacTupleArgs]) time.Time {
	return (&river.DefaultClientRetryPolicy{}).NextRetry(job.JobRow)
}

// applyRebacTuple replays an event using the supplied synchronous authorizer.
// The event model ID is metadata; execution uses the authorizer's configured model.
// Partial application returns an error; replay is safe for duplicate writes and
// missing deletes, provided River preserves permission sequence ordering.
// Terminal skips can leave partial effects that later jobs do not repair.
func applyRebacTuple(ctx context.Context, authorizer rebac.Authorizer, input types.RebacTuple) error {
	if err := workhub.ValidateRebacTuple(input); err != nil {
		return err
	}
	if authorizer == nil {
		return fmt.Errorf("tuple authorizer is required")
	}
	for i, operation := range input.Operations {
		// OpenFGA rejects duplicate keys inside one request even with idempotent writes.
		relationships := uniqueRelationships(operation.Relationships)
		for start := 0; start < len(relationships); start += rebac.DefaultMaxBatchSize {
			end := min(start+rebac.DefaultMaxBatchSize, len(relationships))
			batch := relationships[start:end]
			var err error
			if operation.Delete {
				err = authorizer.Delete(ctx, batch)
			} else {
				err = authorizer.Write(ctx, batch)
			}
			if err != nil {
				return fmt.Errorf("apply tuple operation %d batch %d: %w", i, start/rebac.DefaultMaxBatchSize, err)
			}
		}
	}
	return nil
}

// uniqueRelationships preserves the event order while removing repeated tuple keys.
func uniqueRelationships(input []rebac.Relationship) []rebac.Relationship {
	seen := make(map[rebac.Relationship]struct{}, len(input))
	result := make([]rebac.Relationship, 0, len(input))
	for _, relationship := range input {
		if _, ok := seen[relationship]; ok {
			continue
		}
		seen[relationship] = struct{}{}
		result = append(result, relationship)
	}
	return result
}
