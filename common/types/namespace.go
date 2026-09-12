package types

const (
	OpenCSGPrefix     = "CSG_"
	HuggingfacePrefix = "HF_"

	// UserNamespaceType identifies a namespace owned by an individual user.
	UserNamespaceType = "user"
	// OrganizationNamespaceType identifies a namespace owned by an organization.
	OrganizationNamespaceType = "organization"
)

type Namespace struct {
	Path string
	// Type identifies whether the namespace belongs to a user or an organization.
	Type   string
	Avatar string
	UUID   string
	NSType string
	User   User
}

// WritableNamespace identifies a namespace where the current user can create repositories.
type WritableNamespace struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Name string `json:"name"`
	UUID string `json:"uuid"`
}
