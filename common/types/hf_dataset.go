package types

import "time"

type HFDatasetReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Ref         string `json:"ref"`
	CurrentUser string `json:"current_user"`
}

type HFDatasetMeta struct {
	ID           string      `json:"id"`
	Author       string      `json:"author,omitempty"`
	Sha          string      `json:"sha"`
	Private      bool        `json:"private"`
	Disabled     bool        `json:"disabled,omitempty"`
	Gated        interface{} `json:"gated,omitempty"` // "auto", "manual", or false
	Downloads    int         `json:"downloads"`
	Likes        int         `json:"likes"`
	Tags         []string    `json:"tags"`
	Siblings     []SDKFile   `json:"siblings"`
	CreatedAt    time.Time   `json:"created_at,omitempty"`
	LastModified time.Time   `json:"last_modified,omitempty"`
}

type PathReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Ref         string `json:"ref"`
	Path        string `json:"path"`
	Expand      bool   `json:"expand"`
	CurrentUser string `json:"current_user"`
}

type HFDSPathInfo struct {
	Type       string      `json:"type"`
	Path       string      `json:"path"`
	Size       int64       `json:"size"`
	OID        string      `json:"oid"`
	Lfs        interface{} `json:"lfs,omitempty"`
	LastCommit interface{} `json:"last_commit,omitempty"`
	Security   interface{} `json:"security,omitempty"`
}

type HFErrorRes struct {
	Error string `json:"error"`
}
