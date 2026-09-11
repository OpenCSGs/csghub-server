package rebac

// Relation identifies a direct relationship that can be stored as a tuple.
type Relation string

// Permission identifies a computed permission used by authorization checks.
type Permission string

// CheckRelation represents a direct relation or computed permission accepted by authorization queries.
// The unexported marker limits implementations to types declared by this package.
type CheckRelation interface {
	String() string
	isCheckRelation()
}

// String returns the direct relation name.
func (r Relation) String() string {
	return string(r)
}

// isCheckRelation marks Relation as valid for authorization queries.
func (Relation) isCheckRelation() {}

// String returns the computed permission name.
func (p Permission) String() string {
	return string(p)
}

// isCheckRelation marks Permission as valid for authorization queries.
func (Permission) isCheckRelation() {}

const (
	// RelationOwner identifies the owner subject.
	RelationOwner Relation = "owner"
	// RelationAdmin identifies an administrator subject.
	RelationAdmin Relation = "admin"
	// RelationWriter identifies a subject with a writer-level membership relationship.
	RelationWriter Relation = "writer"
	// RelationReader identifies a subject with a reader-level membership relationship.
	RelationReader Relation = "reader"
	// RelationMember identifies membership in a group-like object.
	RelationMember Relation = "member"
	// RelationMemberFromChild identifies membership inherited from child organizations.
	RelationMemberFromChild Relation = "member_from_child"
	// RelationParent identifies the parent object used for permission inheritance.
	RelationParent Relation = "parent"
	// RelationChild identifies a direct child object in a hierarchy.
	RelationChild Relation = "child"
	// RelationOrganization identifies the organization associated with a resource.
	RelationOrganization Relation = "organization"
	// RelationOrganizationDirect identifies an organization association without parent inheritance.
	RelationOrganizationDirect Relation = "organization_direct"
	// RelationNamespace identifies the namespace associated with a resource.
	RelationNamespace Relation = "namespace"
	// RelationPublic identifies public read access for all user subjects.
	RelationPublic Relation = "public"
)

const (
	// PermissionCanRead permits reading an object.
	PermissionCanRead Permission = "can_read"
	// PermissionCanWrite permits writing object content.
	PermissionCanWrite Permission = "can_write"
	// PermissionCanAdmin permits administrative operations on an object.
	PermissionCanAdmin Permission = "can_admin"
	// PermissionCanPlatformManage permits managing platform-level resources.
	PermissionCanPlatformManage Permission = "can_platform_manage"
	// PermissionCanExecute permits executing an object or workload.
	PermissionCanExecute Permission = "can_execute"
)

const (
	// PlatformCanManage is the platform management permission.
	PlatformCanManage = PermissionCanPlatformManage

	// UserCanWrite permits modifying a user account.
	UserCanWrite = PermissionCanWrite

	// OrganizationCanRead permits reading an organization.
	OrganizationCanRead = PermissionCanRead
	// OrganizationCanWrite permits modifying organization metadata.
	OrganizationCanWrite = PermissionCanWrite
	// OrganizationCanAdmin permits administering an organization.
	OrganizationCanAdmin = PermissionCanAdmin

	// NamespaceCanRead permits reading a namespace.
	NamespaceCanRead = PermissionCanRead
	// NamespaceCanWrite permits writing to a namespace.
	NamespaceCanWrite = PermissionCanWrite
	// NamespaceCanAdmin permits administering a namespace.
	NamespaceCanAdmin = PermissionCanAdmin

	// RepositoryCanRead permits reading a repository.
	RepositoryCanRead = PermissionCanRead
	// RepositoryCanWrite permits writing to a repository.
	RepositoryCanWrite = PermissionCanWrite
	// RepositoryCanAdmin permits administering a repository.
	RepositoryCanAdmin = PermissionCanAdmin

	// KnowledgeBaseCanRead permits reading a knowledge base.
	KnowledgeBaseCanRead = PermissionCanRead
	// KnowledgeBaseCanWrite permits writing knowledge base content.
	KnowledgeBaseCanWrite = PermissionCanWrite
	// KnowledgeBaseCanAdmin permits administering a knowledge base record.
	KnowledgeBaseCanAdmin = PermissionCanAdmin
)
