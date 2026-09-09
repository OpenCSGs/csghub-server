package rebac

import (
	"context"
	"time"
)

// Operation identifies an Authorizer API operation.
type Operation string

const (
	// OperationWrite identifies a relationship tuple write.
	OperationWrite Operation = "write"
	// OperationDelete identifies a relationship tuple deletion.
	OperationDelete Operation = "delete"
	// OperationCheck identifies a check that does not force a denial error.
	OperationCheck Operation = "check"
	// OperationAuthorize identifies an authorization check that enforces denial.
	OperationAuthorize Operation = "authorize"
	// OperationBatchCheck identifies a batch authorization check.
	OperationBatchCheck Operation = "batch_check"
	// OperationListObjects identifies an object listing query with authorization constraints.
	OperationListObjects Operation = "list_objects"
	// OperationListSubjects identifies a subject listing query with authorization constraints.
	OperationListSubjects Operation = "list_subjects"
)

// Observation contains low-cardinality authorization telemetry.
// It intentionally omits subject and object identifiers to avoid high-cardinality data.
type Observation struct {
	Operation    Operation
	Provider     string
	ObjectType   ObjectType
	Relation     CheckRelation
	Duration     time.Duration
	DecisionMade bool
	Allowed      bool
	BatchSize    int
	AllowedCount int
	DeniedCount  int
	ErrorCount   int
	ErrorClass   ErrorClass
}

// Observer receives telemetry for completed authorization operations.
type Observer interface {
	Observe(ctx context.Context, observation Observation)
}

// ObserverFunc adapts a function to the Observer interface.
type ObserverFunc func(ctx context.Context, observation Observation)

// Observe implements the Observer interface.
func (f ObserverFunc) Observe(ctx context.Context, observation Observation) {
	if f != nil {
		f(ctx, observation)
	}
}

type noopObserver struct{}

func (noopObserver) Observe(context.Context, Observation) {}

// NoopObserver returns an Observer that discards all telemetry.
func NoopObserver() Observer {
	return noopObserver{}
}
