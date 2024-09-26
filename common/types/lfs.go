package types

import "time"

type LfsLockReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	CurrentUser string         `json:"current_user"`
	Path        string         `json:"path"`
}

type LFSLockError struct {
	Message       string   `json:"message"`
	Lock          *LFSLock `json:"lock,omitempty"`
	Documentation string   `json:"documentation_url,omitempty"`
	RequestID     string   `json:"request_id,omitempty"`
}

type LFSLock struct {
	ID       string        `json:"id"`
	Path     string        `json:"path"`
	LockedAt time.Time     `json:"locked_at"`
	Owner    *LFSLockOwner `json:"owner"`
}

type LFSLockOwner struct {
	Name string `json:"name"`
}

type LFSLockResponse struct {
	Lock *LFSLock `json:"lock"`
}

type ListLFSLockReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	CurrentUser string         `json:"current_user"`
	Path        string         `json:"path"`
	Cursor      int            `json:"cursor"`
	Limit       int            `json:"limit"`
	ID          int64          `json:"id"`
}

type LFSLockList struct {
	Locks []*LFSLock `json:"locks"`
	Next  string     `json:"next_cursor,omitempty"`
}

type UnlockLFSReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	CurrentUser string         `json:"current_user"`
	Force       bool           `json:"force"`
	ID          int64          `json:"id"`
}

type VerifyLFSLockReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	CurrentUser string         `json:"current_user"`
	Cursor      int            `json:"cursor"`
	Limit       int            `json:"limit"`
}

type LFSLockListVerify struct {
	Ours   []*LFSLock `json:"ours"`
	Theirs []*LFSLock `json:"theirs"`
	Next   string     `json:"next_cursor,omitempty"`
}

type LFSDownload struct {
	Oid         string `json:"oid"`
	DownloadURL string `json:"download_url"`
}

type LFSBatchResponse struct {
	Objects []struct {
		Oid     string `json:"oid"`
		Size    int64  `json:"size"`
		Actions struct {
			Download struct {
				Href string `json:"href"`
			} `json:"download"`
		} `json:"actions"`
	} `json:"objects"`
}

type LFSBatchRequest struct {
	Operation string            `json:"operation"`
	Objects   []LFSBatchObject  `json:"objects"`
	Ref       LFSBatchObjectRef `json:"ref"`
	Transfers []string          `json:"transfers,omitempty"`
	HashAlog  string            `json:"hash_alog,omitempty"`
}

type LFSBatchObjectRef struct {
	Name string `json:"name"`
}

type LFSBatchObject struct {
	Oid  string `json:"oid"`
	Size int64  `json:"size"`
}
