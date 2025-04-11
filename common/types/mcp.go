package types

import "time"

type MCPPropertyKind string

const (
	MCPPropTool             MCPPropertyKind = "tool"
	MCPPropPrompt           MCPPropertyKind = "prompt"
	MCPPropResource         MCPPropertyKind = "resource"
	MCPPropresourceTemplate MCPPropertyKind = "resource_template"
)

type CreateMCPServerReq struct {
	CreateRepoReq
	Configuration string `json:"configuration"`
}

type UpdateMCPServerReq struct {
	UpdateRepoReq
	Configuration *string `json:"configuration"`
}

type MCPServer struct {
	ID                   int64                `json:"id"`
	Name                 string               `json:"name"`
	Nickname             string               `json:"nickname"`
	Description          string               `json:"description"`
	Likes                int64                `json:"likes"`
	Downloads            int64                `json:"downloads"`
	Path                 string               `json:"path"`
	RepositoryID         int64                `json:"repository_id"`
	Repository           Repository           `json:"repository"`
	Private              bool                 `json:"private"`
	User                 User                 `json:"user"`
	Tags                 []RepoTag            `json:"tags"`
	DefaultBranch        string               `json:"default_branch"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
	UserLikes            bool                 `json:"user_likes"`
	Source               RepositorySource     `json:"source"`
	SyncStatus           RepositorySyncStatus `json:"sync_status"`
	License              string               `json:"license"`
	CanWrite             bool                 `json:"can_write"`
	CanManage            bool                 `json:"can_manage"`
	Namespace            *Namespace           `json:"namespace"`
	SensitiveCheckStatus string               `json:"sensitive_check_status"`
	RecomOpWeight        int                  `json:"recom_op_weight,omitempty"`
	Scores               []WeightScore        `json:"scores"`
	ToolsNum             int                  `json:"tools_num"`
	Configuration        string               `json:"configuration"`
	Schema               string               `json:"schema"`
	StarNum              int                  `json:"star_num"`
}

type MCPPropertyFilter struct {
	CurrentUser string          `json:"-"`
	Kind        MCPPropertyKind `json:"kind"`
	Search      string          `json:"search"`
	Per         int             `json:"per"`
	Page        int             `json:"page"`
	IsAdmin     bool            `json:"-"`
	UserIDs     []int64         `json:"-"`
}

type MCPServerProperties struct {
	ID           int64     `json:"id"`
	MCPServerID  int64     `json:"mcp_server_id"`
	RepositoryID int64     `json:"repository_id"`
	Kind         string    `json:"kind"` // tool, prompt, resource, resource_template
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Schema       string    `json:"schema"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	RepoPath     string    `json:"repo_path"`
	Tags         []RepoTag `json:"tags"`
}
