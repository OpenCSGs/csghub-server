package types

type File struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Lfs    bool   `json:"lfs"`
	Size   int    `json:"size"`
	Commit Commit `json:"commit"`
}
