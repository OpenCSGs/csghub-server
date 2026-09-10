package rebac

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPermissionNamingConvention(t *testing.T) {
	permissions := []Permission{
		PermissionCanRead,
		PermissionCanWrite,
		PermissionCanAdmin,
		PermissionCanPlatformManage,
		PermissionCanExecute,
	}
	for _, permission := range permissions {
		require.True(t, strings.HasPrefix(string(permission), "can_"), permission)
	}
	require.Equal(t, PermissionCanWrite, RepositoryCanWrite)
	require.Equal(t, "can_read", PermissionCanRead.String())

	var checkRelation CheckRelation = PermissionCanRead
	require.Equal(t, PermissionCanRead, checkRelation)
}

func TestDirectRelationNamingConvention(t *testing.T) {
	relations := []Relation{
		RelationOwner,
		RelationAdmin,
		RelationWriter,
		RelationReader,
		RelationMember,
		RelationMemberFromChild,
		RelationParent,
		RelationChild,
		RelationOrganization,
		RelationOrganizationDirect,
	}
	for _, relation := range relations {
		require.False(t, strings.HasPrefix(string(relation), "can_"), relation)
	}
	require.Equal(t, "reader", RelationReader.String())

	var checkRelation CheckRelation = RelationReader
	require.Equal(t, RelationReader, checkRelation)
}
