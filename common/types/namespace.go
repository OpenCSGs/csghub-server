package types

const (
	OpenCSGPrefix     = "CSG_"
	HuggingfacePrefix = "HF_"
)

type Namespace struct {
	Path string
	// namespace types like 'user' for normal user, and 'school', 'company' for orgs etc.
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
