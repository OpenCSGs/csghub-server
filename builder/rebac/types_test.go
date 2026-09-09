package rebac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderNeutralRequestTypes(t *testing.T) {
	relationship := Relationship{
		Subject:  UserSubject("user-1"),
		Relation: RelationReader,
		Object:   RepositoryObject(42),
	}
	request := CheckRequest{
		Subject:                 UserSubject("user-2"),
		Relation:                RepositoryCanRead,
		Object:                  RepositoryObject(42),
		ConditionContext:        map[string]any{"ip": "127.0.0.1"},
		ContextualRelationships: []Relationship{relationship},
		Consistency:             ConsistencyHigher,
	}

	require.Equal(t, "user-2", request.Subject.ID)
	require.Equal(t, RepositoryCanRead, request.Relation)
	require.Equal(t, "42", request.Object.ID)
	require.Equal(t, ConsistencyHigher, request.Consistency)
	require.Len(t, request.ContextualRelationships, 1)
}
