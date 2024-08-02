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
}
