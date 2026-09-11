package rebac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalRelationshipStrings(t *testing.T) {
	user := UserSubject("user-1")
	members := OrganizationMembers("org-1")
	membersFromChild := OrganizationMembersFromChild("org-1")
	repository := RepositoryObject(42)
	relationship := Relationship{Subject: members, Relation: RelationReader, Object: repository}

	require.Equal(t, "user:user-1", user.String())
	require.False(t, user.IsUserset())
	require.Equal(t, "organization:org-1#member", members.String())
	require.True(t, members.IsUserset())
	require.Equal(t, "organization:org-1#member_from_child", membersFromChild.String())
	require.True(t, membersFromChild.IsUserset())
	require.Equal(t, "repository:42#reader@organization:org-1#member", relationship.String())
	directRelationship := Relationship{
		Subject:  NewSubject(ObjectTypeOrganization, "org-1"),
		Relation: RelationOrganizationDirect,
		Object:   repository,
	}
	require.Equal(t, "repository:42#organization_direct@organization:org-1", directRelationship.String())
	require.True(t, PublicUserWildcard().IsWildcard())
}
