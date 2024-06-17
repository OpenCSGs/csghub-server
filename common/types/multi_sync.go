package types

import (
	"time"
)

type SyncVersionResponse struct {
	Versions []SyncVersion `json:"versions"`
	HasMore  bool          `json:"has_more"`
}

type SyncVersion struct {
	Version int64 `json:"version"`
	//0: opencsg, 1: HF
	SourceID       int64          `json:"source_id"`
	RepoPath       string         `json:"repo_path"`
	RepoType       RepositoryType `json:"repo_type"`
	LastModifyTime time.Time      `json:"last_modify_time"`
	ChangeLog      string         `json:"change_log"`
}
