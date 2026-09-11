package rebac

import "strconv"

// NewSubject creates a concrete subject without performing I/O.
func NewSubject(objectType ObjectType, id string) Subject {
	return Subject{Type: objectType, ID: id}
}

// NewUserset creates an object#relation userset subject.
func NewUserset(objectType ObjectType, id string, relation Relation) Subject {
	return Subject{Type: objectType, ID: id, Relation: relation}
}

// NewWildcardSubject creates an OpenFGA-compatible typed wildcard subject.
func NewWildcardSubject(objectType ObjectType) Subject {
	return Subject{Type: objectType, ID: WildcardSubjectID}
}

// NewObject creates a protected object without performing I/O.
func NewObject(objectType ObjectType, id string) Object {
	return Object{Type: objectType, ID: id}
}

// UserSubject creates a concrete user subject from a stable user UUID.
func UserSubject(uuid string) Subject {
	return NewSubject(ObjectTypeUser, uuid)
}

// AnonymousSubject creates the reserved subject used for public reads.
func AnonymousSubject() Subject {
	return UserSubject(AnonymousSubjectID)
}

// PublicUserWildcard creates a typed user wildcard for public relationships.
func PublicUserWildcard() Subject {
	return NewWildcardSubject(ObjectTypeUser)
}

// SystemObject creates the platform singleton object.
func SystemObject() Object {
	return NewObject(ObjectTypePlatform, PlatformObjectID)
}

// UserObject creates a protected user account object.
func UserObject(uuid string) Object {
	return NewObject(ObjectTypeUser, uuid)
}

// OrganizationObject creates an organization object.
func OrganizationObject(uuid string) Object {
	return NewObject(ObjectTypeOrganization, uuid)
}

// OrganizationMembers creates the organization member userset.
func OrganizationMembers(uuid string) Subject {
	return NewUserset(ObjectTypeOrganization, uuid, RelationMember)
}

// OrganizationMembersFromChild creates the organization member userset including child organization members.
func OrganizationMembersFromChild(uuid string) Subject {
	return NewUserset(ObjectTypeOrganization, uuid, RelationMemberFromChild)
}

// NamespaceObject creates a namespace object.
func NamespaceObject(uuid string) Object {
	return NewObject(ObjectTypeNamespace, uuid)
}

// RepositoryObject creates a repository object from its immutable database ID.
func RepositoryObject(id int64) Object {
	if id <= 0 {
		return NewObject(ObjectTypeRepository, "")
	}
	return NewObject(ObjectTypeRepository, strconv.FormatInt(id, 10))
}

// KnowledgeBaseObject creates a knowledge base object from its immutable database ID.
func KnowledgeBaseObject(id int64) Object {
	if id <= 0 {
		return NewObject(ObjectTypeKnowledgeBase, "")
	}
	return NewObject(ObjectTypeKnowledgeBase, strconv.FormatInt(id, 10))
}
