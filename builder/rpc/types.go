package rpc

type Namespace struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Avatar string `json:"avatar,omitempty"`
}
