package types

import (
	"time"
)

type MCPPropertyKind string

const (
	MCPPropTool             MCPPropertyKind = "tool"
	MCPPropPrompt           MCPPropertyKind = "prompt"
	MCPPropResource         MCPPropertyKind = "resource"
	MCPPropresourceTemplate MCPPropertyKind = "resource_template"
)

var (
	MCPSpaceConfFileName string = "mcp_space_conf.json"
	MCPSpacePypiKey      string = "PYPI_INDEX_URL"
)

type CreateMCPServerReq struct {
	CreateRepoReq
	Configuration string `json:"configuration"`
}

type UpdateMCPServerReq struct {
	UpdateRepoReq
	Configuration   *string `json:"configuration"`
	ProgramLanguage *string `json:"program_language"`
	RunMode         *string `json:"run_mode"`
	InstallDepsCmds *string `json:"install_deps_cmds"`
	BuildCmds       *string `json:"build_cmds"`
	LaunchCmds      *string `json:"launch_cmds"`
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
	GithubPath           string               `json:"github_path"` // github path
	Readme               string               `json:"readme"`
	MultiSource
	ProgramLanguage  string           `json:"program_language"`
	RunMode          string           `json:"run_mode"`
	InstallDepsCmds  string           `json:"install_deps_cmds"`
	BuildCmds        string           `json:"build_cmds"`
	LaunchCmds       string           `json:"launch_cmds"`
	AvatarURL        string           `json:"avatar_url"`
	MirrorTaskStatus MirrorTaskStatus `json:"mirror_task_status"`
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
	ID           int64           `json:"id"`
	MCPServerID  int64           `json:"mcp_server_id"`
	RepositoryID int64           `json:"repository_id"`
	Kind         MCPPropertyKind `json:"kind"` // tool, prompt, resource, resource_template
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Schema       string          `json:"schema"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	RepoPath     string          `json:"repo_path"`
	Tags         []RepoTag       `json:"tags"`
}

type DeployMCPServerReq struct {
	CurrentUser string      `json:"-"`
	MCPRepo     RepoRequest `json:"-"`
	CreateRepoReq
	CoverImageUrl string `json:"cover_image_url"`
	Env           string `json:"env"`
	ResourceID    int64  `json:"resource_id"` // space resource id
	ClusterID     string `json:"cluster_id"`
}

type MCPSpaceConfig struct {
	ProgramLanguage string `json:"program_language"`
	RunMode         string `json:"run_mode"`
	InstallDepsCmds string `json:"install_deps_cmds"`
	BuildCmds       string `json:"build_cmds"`
	LaunchCmds      string `json:"launch_cmds"`
}

type MCPFilter struct {
	Username string
	Per      int
	Page     int
}
