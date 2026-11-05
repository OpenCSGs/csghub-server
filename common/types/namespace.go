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

// NamespaceChoice preserves the namespace identifier shape needed by recipient selectors.
// It excludes embedded user profiles and unrelated namespace metadata.
type NamespaceChoice struct {
	Path string `json:"Path"`
	Type string `json:"Type"`
	UUID string `json:"UUID"`
}
