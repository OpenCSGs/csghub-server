package rebac

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestValidator(t *testing.T) *Validator {
	t.Helper()
	schema, err := DefaultSchema()
	require.NoError(t, err)
	validator, err := NewValidator(schema)
	require.NoError(t, err)
	return validator
}

func TestValidatorAcceptsValidRequests(t *testing.T) {
	validator := newTestValidator(t)
	request := validCheckRequest()
	request.ConditionContext = map[string]any{
		"ip":      "127.0.0.1",
		"attempt": 2,
		"flags":   []any{true, "trusted"},
	}
	request.ContextualRelationships = []Relationship{
		{
			Subject:  OrganizationMembers("org-1"),
			Relation: RelationReader,
			Object:   RepositoryObject(42),
		},
	}
	require.NoError(t, validator.ValidateCheckRequest(request))

	request.Relation = RelationReader
	require.NoError(t, validator.ValidateCheckRequest(request))
}

func TestValidatorRejectsInvalidSubjectsAndObjects(t *testing.T) {
	validator := newTestValidator(t)

	tests := []struct {
		name    string
		request CheckRequest
		target  error
	}{
		{name: "empty subject", request: CheckRequest{Subject: UserSubject(""), Relation: RepositoryCanRead, Object: RepositoryObject(1)}, target: ErrInvalidSubject},
		{name: "subject separator", request: CheckRequest{Subject: UserSubject("user:1"), Relation: RepositoryCanRead, Object: RepositoryObject(1)}, target: ErrInvalidSubject},
		{name: "wildcard userset", request: CheckRequest{Subject: NewUserset(ObjectTypeUser, WildcardSubjectID, RelationOwner), Relation: RepositoryCanRead, Object: RepositoryObject(1)}, target: ErrInvalidSubject},
		{name: "empty object", request: CheckRequest{Subject: UserSubject("user-1"), Relation: RepositoryCanRead, Object: RepositoryObject(0)}, target: ErrInvalidObject},
		{name: "unknown object type", request: CheckRequest{Subject: UserSubject("user-1"), Relation: RepositoryCanRead, Object: NewObject("unknown", "1")}, target: ErrUnsupportedObjectType},
		{name: "unsupported relation", request: CheckRequest{Subject: UserSubject("user-1"), Relation: RelationMember, Object: RepositoryObject(1)}, target: ErrUnsupportedRelation},
		{name: "permission represented as relation", request: CheckRequest{Subject: UserSubject("user-1"), Relation: Relation(RepositoryCanRead), Object: RepositoryObject(1)}, target: ErrInvalidRelation},
		{name: "relation represented as permission", request: CheckRequest{Subject: UserSubject("user-1"), Relation: Permission(RelationReader), Object: RepositoryObject(1)}, target: ErrInvalidRelation},
		{name: "nil query relation", request: CheckRequest{Subject: UserSubject("user-1"), Object: RepositoryObject(1)}, target: ErrInvalidRelation},
		{name: "invalid consistency", request: CheckRequest{Subject: UserSubject("user-1"), Relation: RepositoryCanRead, Object: RepositoryObject(1), Consistency: Consistency(200)}, target: ErrInvalidRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validator.ValidateCheckRequest(test.request)
			require.ErrorIs(t, err, test.target)
		})
	}
}

func TestValidatorRejectsInvalidConditionContext(t *testing.T) {
	validator := newTestValidator(t)

	request := validCheckRequest()
	request.ConditionContext = map[string]any{"invalid": struct{}{}}
	require.ErrorIs(t, validator.ValidateCheckRequest(request), ErrInvalidRequest)

	request = validCheckRequest()
	request.ConditionContext = map[string]any{"invalid": math.NaN()}
	require.ErrorIs(t, validator.ValidateCheckRequest(request), ErrInvalidRequest)

	request = validCheckRequest()
	request.ConditionContext = map[string]any{"large": strings.Repeat("x", DefaultMaxConditionContextBytes)}
	require.ErrorIs(t, validator.ValidateCheckRequest(request), ErrInvalidRequest)

	root := map[string]any{}
	cursor := root
	for index := 0; index <= DefaultMaxConditionContextDepth; index++ {
		next := map[string]any{}
		cursor["next"] = next
		cursor = next
	}
	request = validCheckRequest()
	request.ConditionContext = root
	require.ErrorIs(t, validator.ValidateCheckRequest(request), ErrInvalidRequest)
}

func TestValidatorRejectsComputedContextualRelationship(t *testing.T) {
	validator := newTestValidator(t)
	request := validCheckRequest()
	request.ContextualRelationships = []Relationship{
		{
			Subject:  UserSubject("user-1"),
			Relation: Relation(RepositoryCanRead),
			Object:   RepositoryObject(42),
		},
	}
	require.ErrorIs(t, validator.ValidateCheckRequest(request), ErrInvalidRelation)
}

func TestValidatorValidatesRelationshipBatches(t *testing.T) {
	validator := newTestValidator(t)
	relationships := []Relationship{{
		Subject:  UserSubject("user-1"),
		Relation: RelationReader,
		Object:   RepositoryObject(42),
	}}
	require.NoError(t, validator.ValidateRelationships(relationships))
	require.ErrorIs(t, validator.ValidateRelationships(nil), ErrInvalidRequest)

	relationships[0].Relation = Relation(RepositoryCanRead)
	require.ErrorIs(t, validator.ValidateRelationships(relationships), ErrInvalidRelation)
}

func TestValidatorValidatesBatchAndListRequests(t *testing.T) {
	validator := newTestValidator(t)
	check := validCheckRequest()

	require.NoError(t, validator.ValidateBatchCheckRequest(BatchCheckRequest{Checks: []BatchCheckItem{
		{CorrelationID: "one", Check: check},
		{CorrelationID: "two", Check: check},
	}}, DefaultMaxBatchSize))
	require.ErrorIs(t, validator.ValidateBatchCheckRequest(BatchCheckRequest{}, DefaultMaxBatchSize), ErrInvalidRequest)
	require.ErrorIs(t, validator.ValidateBatchCheckRequest(BatchCheckRequest{Checks: []BatchCheckItem{
		{CorrelationID: "same", Check: check},
		{CorrelationID: "same", Check: check},
	}}, DefaultMaxBatchSize), ErrDuplicateCorrelationID)
	require.ErrorIs(t, validator.ValidateBatchCheckRequest(BatchCheckRequest{Checks: []BatchCheckItem{
		{CorrelationID: "one", Check: check},
		{CorrelationID: "two", Check: check},
	}}, 1), ErrBatchLimitExceeded)
	for _, correlationID := range []string{
		"",
		"namespace:0",
		"contains space",
		"中文",
		strings.Repeat("a", DefaultMaxCorrelationIDBytes+1),
	} {
		require.ErrorIs(t, validator.ValidateBatchCheckRequest(BatchCheckRequest{Checks: []BatchCheckItem{{
			CorrelationID: correlationID,
			Check:         check,
		}}}, DefaultMaxBatchSize), ErrInvalidRequest)
	}
	require.NoError(t, validator.ValidateBatchCheckRequest(BatchCheckRequest{Checks: []BatchCheckItem{{
		CorrelationID: strings.Repeat("a", DefaultMaxCorrelationIDBytes),
		Check:         check,
	}}}, DefaultMaxBatchSize))
	require.Equal(t, "c49", BatchCheckCorrelationID(49))

	require.NoError(t, validator.ValidateListObjectsRequest(ListObjectsRequest{
		Subject:    UserSubject("user-1"),
		Relation:   RepositoryCanRead,
		ObjectType: ObjectTypeRepository,
	}))
	require.NoError(t, validator.ValidateListSubjectsRequest(ListSubjectsRequest{
		Object:      RepositoryObject(42),
		Relation:    RepositoryCanRead,
		SubjectType: ObjectTypeUser,
	}))
}
