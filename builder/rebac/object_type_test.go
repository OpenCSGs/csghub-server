package rebac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommonObjectTypesAreStableAndUnique(t *testing.T) {
	types := []ObjectType{
		ObjectTypePlatform,
		ObjectTypeUser,
		ObjectTypeOrganization,
		ObjectTypeNamespace,
		ObjectTypeRepository,
		ObjectTypeKnowledgeBase,
	}
	seen := make(map[ObjectType]struct{}, len(types))
	for _, objectType := range types {
		require.NotEmpty(t, objectType)
		_, exists := seen[objectType]
		require.False(t, exists)
		seen[objectType] = struct{}{}
	}
	require.Equal(t, "platform", PlatformObjectID)
	require.Equal(t, "anonymous", AnonymousSubjectID)
	require.Equal(t, "*", WildcardSubjectID)
}
