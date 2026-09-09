package rebac

import "strconv"

// BatchCheckCorrelationID returns a short, batch-local identifier accepted by OpenFGA.
// Callers must use a unique index within each BatchCheck request.
func BatchCheckCorrelationID(index int) string {
	return "c" + strconv.Itoa(index)
}

// Subject represents an authenticated subject, a typed wildcard, or a userset.
// A concrete subject has an empty Relation; an object#relation userset carries its relation.
type Subject struct {
	Type     ObjectType
	ID       string
	Relation Relation
}

// Object represents a protected resource.
type Object struct {
	Type ObjectType
	ID   string
}

// CheckRequest asks whether Subject has the requested direct relation or computed permission on Object.
type CheckRequest struct {
	Subject                 Subject
	Relation                CheckRelation
	Object                  Object
	ConditionContext        map[string]any
	ContextualRelationships []Relationship
	Consistency             Consistency
}

// Decision is an authorization result independent of a specific Provider.
type Decision struct {
	Allowed bool
}

// BatchCheckItem identifies one authorization check within a request by correlation ID.
type BatchCheckItem struct {
	CorrelationID string
	Check         CheckRequest
}

// BatchCheckRequest contains multiple independent authorization checks.
type BatchCheckRequest struct {
	Checks []BatchCheckItem
}

// BatchCheckOutcome contains one authorization decision or its corresponding error.
type BatchCheckOutcome struct {
	Decision Decision
	Err      error
}

// BatchCheckResult indexes each check outcome by correlation ID.
type BatchCheckResult struct {
	Results map[string]BatchCheckOutcome
}

// ListObjectsRequest asks for objects on which Subject has the requested relation or permission.
type ListObjectsRequest struct {
	Subject                 Subject
	Relation                CheckRelation
	ObjectType              ObjectType
	ConditionContext        map[string]any
	ContextualRelationships []Relationship
	Consistency             Consistency
}

// ListObjectsResult contains only authorization object references.
type ListObjectsResult struct {
	Objects []Object
}

// ListSubjectsRequest asks for subjects that have the requested relation or permission on Object.
type ListSubjectsRequest struct {
	Object                  Object
	Relation                CheckRelation
	SubjectType             ObjectType
	ConditionContext        map[string]any
	ContextualRelationships []Relationship
	Consistency             Consistency
}

// ListSubjectsResult contains concrete subjects or usersets.
type ListSubjectsResult struct {
	Subjects []Subject
}
