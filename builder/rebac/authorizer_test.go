package rebac

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewAuthorizerValidatesDependenciesAndOptions(t *testing.T) {
	_, err := NewAuthorizer(nil)
	require.ErrorIs(t, err, ErrInvalidRequest)

	provider := &mockProvider{name: "test"}
	_, err = NewAuthorizer(provider, WithSchema(nil))
	require.ErrorIs(t, err, ErrInvalidSchema)
	_, err = NewAuthorizer(provider, WithObserver(nil))
	require.ErrorIs(t, err, ErrInvalidRequest)
	_, err = NewAuthorizer(provider, WithTimeout(-time.Second))
	require.ErrorIs(t, err, ErrInvalidRequest)
	_, err = NewAuthorizer(provider, WithMaxBatchSize(0))
	require.ErrorIs(t, err, ErrInvalidRequest)
	_, err = NewAuthorizer(provider, nil)
	require.ErrorIs(t, err, ErrInvalidRequest)
}

func TestAuthorizerCheckAndAuthorize(t *testing.T) {
	allowed := true
	provider := &mockProvider{
		name: "database",
		check: func(context.Context, CheckRequest) (Decision, error) {
			return Decision{Allowed: allowed}, nil
		},
	}
	observations := make([]Observation, 0, 2)
	authorizer, err := NewAuthorizer(provider, WithObserver(ObserverFunc(func(_ context.Context, observation Observation) {
		observations = append(observations, observation)
	})))
	require.NoError(t, err)

	decision, err := authorizer.Check(context.Background(), validCheckRequest())
	require.NoError(t, err)
	require.True(t, decision.Allowed)

	allowed = false
	err = authorizer.Authorize(context.Background(), validCheckRequest())
	require.ErrorIs(t, err, ErrDenied)
	require.Equal(t, 2, provider.checkCalls)
	require.Len(t, observations, 2)
	require.Equal(t, OperationCheck, observations[0].Operation)
	require.True(t, observations[0].DecisionMade)
	require.True(t, observations[0].Allowed)
	require.Equal(t, OperationAuthorize, observations[1].Operation)
	require.True(t, observations[1].DecisionMade)
	require.False(t, observations[1].Allowed)
	require.Equal(t, ErrorClassDenied, observations[1].ErrorClass)
}

func TestAuthorizerWriteAndDelete(t *testing.T) {
	relationships := []Relationship{{
		Subject:  UserSubject("user-1"),
		Relation: RelationReader,
		Object:   RepositoryObject(42),
	}}
	provider := &mockProvider{
		name: "openfga",
		write: func(_ context.Context, actual []Relationship) error {
			require.Equal(t, relationships, actual)
			return nil
		},
		delete: func(_ context.Context, actual []Relationship) error {
			require.Equal(t, relationships, actual)
			return nil
		},
	}
	observations := make([]Observation, 0, 2)
	authorizer, err := NewAuthorizer(provider, WithObserver(ObserverFunc(func(_ context.Context, observation Observation) {
		observations = append(observations, observation)
	})))
	require.NoError(t, err)

	require.NoError(t, authorizer.Write(context.Background(), relationships))
	require.NoError(t, authorizer.Delete(context.Background(), relationships))
	require.Equal(t, 1, provider.writeCalls)
	require.Equal(t, 1, provider.deleteCalls)
	require.Len(t, observations, 2)
	require.Equal(t, OperationWrite, observations[0].Operation)
	require.Equal(t, 1, observations[0].BatchSize)
	require.Equal(t, OperationDelete, observations[1].Operation)
	require.Equal(t, 1, observations[1].BatchSize)
}

func TestAuthorizerRejectsInvalidRelationshipMutations(t *testing.T) {
	provider := &mockProvider{name: "openfga"}
	authorizer, err := NewAuthorizer(provider)
	require.NoError(t, err)

	require.ErrorIs(t, authorizer.Write(context.Background(), nil), ErrInvalidRequest)
	require.ErrorIs(t, authorizer.Delete(context.Background(), []Relationship{{
		Subject:  UserSubject("user-1"),
		Relation: Relation(RepositoryCanRead),
		Object:   RepositoryObject(42),
	}}), ErrInvalidRelation)
	//nolint:staticcheck // A nil context must fail closed instead of triggering a panic.
	require.ErrorIs(t, authorizer.Write(nil, []Relationship{{
		Subject:  UserSubject("user-1"),
		Relation: RelationReader,
		Object:   RepositoryObject(42),
	}}), ErrInvalidRequest)
	require.Zero(t, provider.writeCalls)
	require.Zero(t, provider.deleteCalls)
}

func TestAuthorizerRejectsInvalidRequestBeforeProvider(t *testing.T) {
	provider := &mockProvider{name: "database"}
	authorizer, err := NewAuthorizer(provider)
	require.NoError(t, err)

	request := validCheckRequest()
	request.Object = RepositoryObject(0)
	_, err = authorizer.Check(context.Background(), request)
	require.ErrorIs(t, err, ErrInvalidObject)
	require.Zero(t, provider.checkCalls)

	//nolint:staticcheck // A nil context must fail closed instead of triggering a panic.
	_, err = authorizer.Check(nil, validCheckRequest())
	require.ErrorIs(t, err, ErrInvalidRequest)
	require.Zero(t, provider.checkCalls)
}

func TestAuthorizerProviderTimeoutAndContractViolation(t *testing.T) {
	provider := &mockProvider{
		name: "slow",
		check: func(ctx context.Context, _ CheckRequest) (Decision, error) {
			<-ctx.Done()
			return Decision{}, ctx.Err()
		},
	}
	authorizer, err := NewAuthorizer(provider, WithTimeout(time.Millisecond))
	require.NoError(t, err)
	_, err = authorizer.Check(context.Background(), validCheckRequest())
	require.ErrorIs(t, err, ErrProviderTimeout)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	provider.check = func(context.Context, CheckRequest) (Decision, error) {
		return Decision{}, ErrDenied
	}
	authorizer, err = NewAuthorizer(provider)
	require.NoError(t, err)
	_, err = authorizer.Check(context.Background(), validCheckRequest())
	require.ErrorIs(t, err, ErrInvalidProviderResponse)
}

func TestAuthorizerBatchCheck(t *testing.T) {
	provider := &mockProvider{
		name: "database",
		batchCheck: func(_ context.Context, request BatchCheckRequest) (BatchCheckResult, error) {
			return BatchCheckResult{Results: map[string]BatchCheckOutcome{
				request.Checks[0].CorrelationID: {Decision: Decision{Allowed: true}},
				request.Checks[1].CorrelationID: {Decision: Decision{Allowed: false}},
			}}, nil
		},
	}
	var observation Observation
	authorizer, err := NewAuthorizer(provider, WithObserver(ObserverFunc(func(_ context.Context, value Observation) {
		observation = value
	})))
	require.NoError(t, err)
	request := BatchCheckRequest{Checks: []BatchCheckItem{
		{CorrelationID: "allowed", Check: validCheckRequest()},
		{CorrelationID: "denied", Check: validCheckRequest()},
	}}
	result, err := authorizer.BatchCheck(context.Background(), request)
	require.NoError(t, err)
	require.True(t, result.Results["allowed"].Decision.Allowed)
	require.False(t, result.Results["denied"].Decision.Allowed)
	require.Equal(t, 1, observation.AllowedCount)
	require.Equal(t, 1, observation.DeniedCount)
	require.Equal(t, 2, observation.BatchSize)
}

func TestAuthorizerRejectsInvalidBatchProviderResponse(t *testing.T) {
	request := BatchCheckRequest{Checks: []BatchCheckItem{
		{CorrelationID: "missing", Check: validCheckRequest()},
	}}
	tests := []struct {
		name   string
		result BatchCheckResult
	}{
		{
			name:   "missing result",
			result: BatchCheckResult{Results: map[string]BatchCheckOutcome{}},
		},
		{
			name: "allowed result with error",
			result: BatchCheckResult{Results: map[string]BatchCheckOutcome{
				"missing": {Decision: Decision{Allowed: true}, Err: errors.New("provider item failure")},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &mockProvider{
				name: "database",
				batchCheck: func(context.Context, BatchCheckRequest) (BatchCheckResult, error) {
					return test.result, nil
				},
			}
			authorizer, err := NewAuthorizer(provider)
			require.NoError(t, err)
			_, err = authorizer.BatchCheck(context.Background(), request)
			require.ErrorIs(t, err, ErrInvalidProviderResponse)
		})
	}
}

func TestAuthorizerListObjectsAndSubjects(t *testing.T) {
	provider := &mockProvider{
		name: "database",
		listObjects: func(context.Context, ListObjectsRequest) (ListObjectsResult, error) {
			return ListObjectsResult{Objects: []Object{RepositoryObject(1), RepositoryObject(2)}}, nil
		},
		listSubjects: func(context.Context, ListSubjectsRequest) (ListSubjectsResult, error) {
			return ListSubjectsResult{Subjects: []Subject{UserSubject("user-1"), UserSubject("user-2")}}, nil
		},
	}
	authorizer, err := NewAuthorizer(provider)
	require.NoError(t, err)

	objects, err := authorizer.ListObjects(context.Background(), ListObjectsRequest{
		Subject:    UserSubject("user-1"),
		Relation:   RepositoryCanRead,
		ObjectType: ObjectTypeRepository,
	})
	require.NoError(t, err)
	require.Len(t, objects.Objects, 2)

	subjects, err := authorizer.ListSubjects(context.Background(), ListSubjectsRequest{
		Object:      RepositoryObject(1),
		Relation:    RepositoryCanRead,
		SubjectType: ObjectTypeUser,
	})
	require.NoError(t, err)
	require.Len(t, subjects.Subjects, 2)
	require.Equal(t, 1, provider.listObjectCalls)
	require.Equal(t, 1, provider.listSubjectCalls)
}

func TestAuthorizerRejectsInvalidListProviderResponses(t *testing.T) {
	provider := &mockProvider{
		name: "database",
		listObjects: func(context.Context, ListObjectsRequest) (ListObjectsResult, error) {
			return ListObjectsResult{Objects: []Object{NamespaceObject("namespace-1")}}, nil
		},
		listSubjects: func(context.Context, ListSubjectsRequest) (ListSubjectsResult, error) {
			return ListSubjectsResult{Subjects: []Subject{OrganizationMembers("org-1")}}, nil
		},
	}
	authorizer, err := NewAuthorizer(provider)
	require.NoError(t, err)

	_, err = authorizer.ListObjects(context.Background(), ListObjectsRequest{
		Subject:    UserSubject("user-1"),
		Relation:   RepositoryCanRead,
		ObjectType: ObjectTypeRepository,
	})
	require.ErrorIs(t, err, ErrInvalidProviderResponse)

	_, err = authorizer.ListSubjects(context.Background(), ListSubjectsRequest{
		Object:      RepositoryObject(1),
		Relation:    RepositoryCanRead,
		SubjectType: ObjectTypeUser,
	})
	require.ErrorIs(t, err, ErrInvalidProviderResponse)
}

func TestAuthorizerPropagatesProviderErrors(t *testing.T) {
	providerError := errors.New("database unavailable")
	provider := &mockProvider{
		name: "database",
		check: func(context.Context, CheckRequest) (Decision, error) {
			return Decision{}, providerError
		},
	}
	authorizer, err := NewAuthorizer(provider)
	require.NoError(t, err)
	_, err = authorizer.Check(context.Background(), validCheckRequest())
	require.ErrorIs(t, err, providerError)
}
