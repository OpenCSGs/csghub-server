package workhub

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

const (
	// RebacTupleQueue isolates permission projection from long-running jobs.
	RebacTupleQueue = "rebac_tuple_queue"
	// RebacTupleJobTimeout bounds one application attempt.
	RebacTupleJobTimeout = 30 * time.Second
	// RebacTupleMaxAttempts retains events through temporary FGA outages.
	RebacTupleMaxAttempts = 12
	// RebacTupleMaxRelationships caps the total write/delete relationships per River job.
	RebacTupleMaxRelationships = 50
)

// RebacTupleArgs is the versioned snapshot stored in river_job.args.
type RebacTupleArgs struct {
	SubjectKey string `json:"subject_key" river:"sequence"`
	types.RebacTuple
}

// SequenceOpts serializes each permission group and advances past cancelled or discarded jobs.
func (RebacTupleArgs) SequenceOpts() river.SequenceOpts {
	continueOnTerminal := true
	return river.SequenceOpts{ByArgs: true, ContinueOnCancelled: &continueOnTerminal, ContinueOnDiscarded: &continueOnTerminal}
}

// RebacSequenceKey groups concrete subjects and usersets by subject identity.
// Public user wildcard relations use one sequence per object to isolate revocations.
func RebacSequenceKey(relationship rebac.Relationship) string {
	subject := relationship.Subject
	key := string(subject.Type) + ":" + subject.ID
	if subject == rebac.PublicUserWildcard() {
		key += "|" + relationship.Object.String()
	}
	return key
}

// Kind identifies tuple mutation jobs.
func (RebacTupleArgs) Kind() string { return RebacTupleQueue }

// InsertOpts routes tuple events to their dedicated queue.
func (RebacTupleArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: RebacTupleQueue}
}

// rebacTupleJobClient adapts the existing transactional River producer.
type rebacTupleJobClient struct{ client JobClient }

// NewRebacTupleJobClient builds the database-owned enqueue interface.
func NewRebacTupleJobClient(client JobClient) database.RebacTupleJobClient {
	return &rebacTupleJobClient{client: client}
}

// NewRebacTupleJobClientWithDB creates a producer for injection into business Stores.
// An uninitialized database yields no producer; tuple-changing Stores then fail closed.
func NewRebacTupleJobClientWithDB(db *database.DB) (database.RebacTupleJobClient, error) {
	if db == nil || db.BunDB == nil {
		return nil, nil
	}
	client, err := NewJobClient(context.Background(), db.BunDB)
	if err != nil {
		return nil, fmt.Errorf("create ReBAC tuple job client: %w", err)
	}
	return NewRebacTupleJobClient(client), nil
}

// InsertRebacTupleJobsTx validates and inserts every subject chunk in one batch.
func (c *rebacTupleJobClient) InsertRebacTupleJobsTx(ctx context.Context, tx *sql.Tx, inputs []types.RebacTuple) error {
	if len(inputs) == 0 {
		return nil
	}
	if c.client == nil {
		return fmt.Errorf("tuple job client is required")
	}
	if tx == nil {
		return fmt.Errorf("tuple mutation requires a business transaction")
	}
	var params []InsertManyParams
	for _, input := range inputs {
		if err := ValidateRebacTuple(input); err != nil {
			return err
		}
		for _, args := range splitRebacTuple(input) {
			params = append(params, InsertManyParams{Args: args, Opts: &InsertOpts{Queue: RebacTupleQueue, MaxAttempts: RebacTupleMaxAttempts}})
		}
	}
	_, err := c.client.InsertManyTx(ctx, tx, params)
	return err
}

// splitRebacTuple preserves operation order within each sequence and caps job size.
func splitRebacTuple(input types.RebacTuple) []RebacTupleArgs {
	var groups []RebacTupleArgs
	bySubject := make(map[string]int)
	for _, operation := range input.Operations {
		operationIndex := make(map[string]int)
		for _, relationship := range operation.Relationships {
			key := RebacSequenceKey(relationship)
			index, found := bySubject[key]
			if !found {
				index = len(groups)
				bySubject[key] = index
				event := input
				event.Operations = nil
				groups = append(groups, RebacTupleArgs{SubjectKey: key, RebacTuple: event})
			}
			group := &groups[index]
			opIndex, found := operationIndex[key]
			if !found {
				opIndex = len(group.Operations)
				operationIndex[key] = opIndex
				group.Operations = append(group.Operations, types.TupleOperation{Delete: operation.Delete})
			}
			group.Operations[opIndex].Relationships = append(group.Operations[opIndex].Relationships, relationship)
		}
	}
	var jobs []RebacTupleArgs
	for _, group := range groups {
		chunk := group
		chunk.Operations = nil
		count := 0
		for _, operation := range group.Operations {
			remaining := operation.Relationships
			for len(remaining) > 0 {
				n := min(len(remaining), RebacTupleMaxRelationships-count)
				chunk.Operations = append(chunk.Operations, types.TupleOperation{Delete: operation.Delete, Relationships: remaining[:n]})
				remaining = remaining[n:]
				count += n
				if count == RebacTupleMaxRelationships {
					jobs = append(jobs, chunk)
					chunk.Operations = nil
					count = 0
				}
			}
		}
		if count > 0 {
			jobs = append(jobs, chunk)
		}
	}
	return jobs
}

// ValidateRebacTupleJob rejects invalid sequence identities and oversized payloads.
func ValidateRebacTupleJob(args RebacTupleArgs) error {
	if err := ValidateRebacTuple(args.RebacTuple); err != nil {
		return err
	}
	count := 0
	for _, operation := range args.Operations {
		for _, relationship := range operation.Relationships {
			if args.SubjectKey != RebacSequenceKey(relationship) {
				return fmt.Errorf("tuple subject does not match sequence subject %q", args.SubjectKey)
			}
			count++
		}
	}
	if count > RebacTupleMaxRelationships {
		return fmt.Errorf("tuple job has %d relationships, exceeds maximum %d", count, RebacTupleMaxRelationships)
	}
	return nil
}

// ValidateRebacTuple checks the complete event before any external operation takes place.
func ValidateRebacTuple(input types.RebacTuple) error {
	if input.Version != 1 {
		return fmt.Errorf("unsupported tuple event version %d", input.Version)
	}
	if input.ModelID == "" {
		return fmt.Errorf("tuple model ID is required")
	}
	if input.Source == "" || len(input.Operations) == 0 {
		return fmt.Errorf("tuple event requires a source and operations")
	}
	schema, err := rebac.DefaultSchema()
	if err != nil {
		return err
	}
	validator, err := rebac.NewValidator(schema)
	if err != nil {
		return err
	}
	for i, operation := range input.Operations {
		if len(operation.Relationships) == 0 {
			return fmt.Errorf("tuple operation %d is empty", i)
		}
		for _, relationship := range operation.Relationships {
			if err := validator.ValidateRelationships([]rebac.Relationship{relationship}); err != nil {
				return fmt.Errorf("tuple operation %d: %w", i, err)
			}
		}
	}
	return nil
}
