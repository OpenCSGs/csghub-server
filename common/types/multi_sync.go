package types

import (
	"time"
)

type SyncVersionResponse struct {
	Message string `json:"msg"`
	Data    struct {
		Versions []SyncVersion `json:"versions"`
		HasMore  bool          `json:"has_more"`
	}
}

type SyncVersionData struct {
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

type ModelResponse struct {
	Message string `json:"msg"`
	Data    Model  `json:"data"`
}

type DatasetResponse struct {
	Message string  `json:"msg"`
	Data    Dataset `json:"data"`
}

type CodeResponse struct {
	Message string `json:"msg"`
	Data    Code   `json:"data"`
}

type PromptResponse struct {
	Message string    `json:"msg"`
	Data    PromptRes `json:"data"`
}

type MCPServerResponse struct {
	Message string    `json:"msg"`
	Data    MCPServer `json:"data"`
}

const (
	SyncVersionSourceOpenCSG = iota
	SyncVersionSourceHF
)

type ReadMeResponse struct {
	Message string `json:"msg"`
	Data    string `json:"data"`
}

type AllFilesResponse struct {
	Message string `json:"msg"`
	Data    []File `json:"data"`
}

type RemoteDiffReq struct {
	RepoType     RepositoryType `json:"repo_type"`
	Namespace    string         `json:"namespace"`
	Name         string         `json:"name"`
	LeftCommitID string         `json:"left_commit_id"`
}

type RemoteDiffs struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

type RemoteDiffRespones struct {
	Message string        `json:"msg"`
	Data    []RemoteDiffs `json:"data"`
}
