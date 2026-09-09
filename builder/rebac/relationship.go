package rebac

// Relationship represents the standard OpenFGA object#relation@subject tuple.
// Only direct relations such as owner, admin, writer, reader, parent, child,
// organization, and organization_direct can be persisted. Computed permissions such as can_read,
// can_write, and can_admin are evaluated by OpenFGA and must not be written as
// relationship tuples.
//
// A direct user grant on a repository:
//
//	Relationship{
//		Subject:  UserSubject("user-1"),
//		Relation: RelationReader,
//		Object:   RepositoryObject(42),
//	}
//
// represents:
//
//	repository:42#reader@user:user-1
//
// Associating a repository with an organization:
//
//	Relationship{
//		Subject:  NewSubject(ObjectTypeOrganization, "sales"),
//		Relation: RelationOrganization,
//		Object:   RepositoryObject(42),
//	}
//
// represents:
//
//	repository:42#organization@organization:sales
//
// Connecting a child organization to its parent requires two relationships:
//
//	Relationship{
//		Subject:  NewSubject(ObjectTypeOrganization, "sales"),
//		Relation: RelationParent,
//		Object:   OrganizationObject("sales-team-1"),
//	}
//	Relationship{
//		Subject:  NewSubject(ObjectTypeOrganization, "sales-team-1"),
//		Relation: RelationChild,
//		Object:   OrganizationObject("sales"),
//	}
//
// represent:
//
//	organization:sales-team-1#parent@organization:sales
//	organization:sales#child@organization:sales-team-1
type Relationship struct {
	// Subject is the user, organization, wildcard, or userset on the right side of the tuple.
	Subject Subject
	// Relation is the direct relationship on the target object.
	Relation Relation
	// Object is the target resource on the left side of the tuple.
	Object Object
}

// String returns the subject string representation compatible with OpenFGA.
func (s Subject) String() string {
	value := string(s.Type) + ":" + s.ID
	if s.Relation != "" {
		value += "#" + string(s.Relation)
	}
	return value
}

// IsUserset reports whether the subject represents an object#relation userset.
func (s Subject) IsUserset() bool {
	return s.Relation != ""
}

// IsWildcard reports whether the subject represents a typed public wildcard.
func (s Subject) IsWildcard() bool {
	return s.ID == WildcardSubjectID
}

// String returns the object string representation compatible with OpenFGA.
func (o Object) String() string {
	return string(o.Type) + ":" + o.ID
}

// String returns the relationship tuple string representation compatible with OpenFGA.
func (r Relationship) String() string {
	return r.Object.String() + "#" + string(r.Relation) + "@" + r.Subject.String()
}
