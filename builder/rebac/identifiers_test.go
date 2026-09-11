package rebac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypedIdentifierConstructors(t *testing.T) {
	require.Equal(t, Subject{Type: ObjectTypeUser, ID: "user-1"}, UserSubject("user-1"))
	require.Equal(t, Subject{Type: ObjectTypeUser, ID: AnonymousSubjectID}, AnonymousSubject())
	require.Equal(t, Subject{Type: ObjectTypeUser, ID: WildcardSubjectID}, PublicUserWildcard())
	require.Equal(t, Object{Type: ObjectTypePlatform, ID: PlatformObjectID}, SystemObject())
	require.Equal(t, Object{Type: ObjectTypeUser, ID: "user-1"}, UserObject("user-1"))
	require.Equal(t, Object{Type: ObjectTypeOrganization, ID: "org-1"}, OrganizationObject("org-1"))
	require.Equal(t, Subject{Type: ObjectTypeOrganization, ID: "org-1", Relation: RelationMember}, OrganizationMembers("org-1"))
	require.Equal(t, Subject{Type: ObjectTypeOrganization, ID: "org-1", Relation: RelationMemberFromChild}, OrganizationMembersFromChild("org-1"))
	require.Equal(t, Object{Type: ObjectTypeNamespace, ID: "namespace-1"}, NamespaceObject("namespace-1"))
	require.Equal(t, Object{Type: ObjectTypeRepository, ID: "42"}, RepositoryObject(42))
	require.Empty(t, RepositoryObject(0).ID)
}
