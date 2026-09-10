package rebac

import "context"

// Provider executes authorization queries and relationship mutations without handling HTTP-layer concerns.
// For a valid denial, an implementation must return Decision{Allowed: false}, nil.
type Provider interface {
	// Write persists direct relationship tuples.
	Write(ctx context.Context, relationships []Relationship) error
	// Delete removes direct relationship tuples.
	Delete(ctx context.Context, relationships []Relationship) error
	// Check evaluates one subject, relation, and object authorization request.
	Check(ctx context.Context, request CheckRequest) (Decision, error)
	// BatchCheck evaluates independent authorization checks keyed by correlation ID.
	BatchCheck(ctx context.Context, request BatchCheckRequest) (BatchCheckResult, error)
	// ListObjects lists object references accessible to a subject through a relation.
	ListObjects(ctx context.Context, request ListObjectsRequest) (ListObjectsResult, error)
	// ListSubjects lists subjects that have a relation on an object.
	ListSubjects(ctx context.Context, request ListSubjectsRequest) (ListSubjectsResult, error)
	// Name returns a stable, low-cardinality Provider name.
	Name() string
}
