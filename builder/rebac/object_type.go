package rebac

// ObjectType identifies a resource type in the authorization model.
type ObjectType string

const (
	// ObjectTypePlatform identifies a platform-level resource.
	ObjectTypePlatform ObjectType = "platform"
	// ObjectTypeUser identifies a user account as a protected resource.
	ObjectTypeUser ObjectType = "user"
	// ObjectTypeOrganization identifies an organization.
	ObjectTypeOrganization ObjectType = "organization"
	// ObjectTypeNamespace identifies a personal or organization namespace.
	ObjectTypeNamespace ObjectType = "namespace"
	// ObjectTypeRepository identifies the repository object shared by derived repository resources.
	ObjectTypeRepository ObjectType = "repository"
	// ObjectTypeKnowledgeBase identifies an agent knowledge base.
	ObjectTypeKnowledgeBase ObjectType = "knowledge_base"
)

const (
	// PlatformObjectID is the stable identifier of the platform authorization object.
	PlatformObjectID = "platform"
	// AnonymousSubjectID is the reserved subject identifier used for unauthenticated public reads.
	AnonymousSubjectID = "anonymous"
	// WildcardSubjectID is the typed public wildcard identifier compatible with OpenFGA.
	WildcardSubjectID = "*"
)
