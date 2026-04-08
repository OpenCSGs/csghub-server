package types

import "time"

type CreateSkillReq struct {
	CreateRepoReq
	// Skill package SHA256 hash
	SkillPackageSHA256 string `json:"skill_file"`
	// Git repository URL for mirroring
	GitURL string `json:"git_url"`
	// Git username for authentication
	GitUsername string `json:"git_username"`
	// Git password for authentication
	GitPassword string `json:"git_password"`
}

type UpdateSkillReq struct {
	UpdateRepoReq
}

type Skill struct {
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
	Readme               string               `json:"readme"`
	Scores               []WeightScore        `json:"scores"`
	MultiSource
	MirrorTaskStatus MirrorTaskStatus `json:"mirror_task_status"`
	RepoSize         int64            `json:"repo_size"`
}

type OrgSkillsReq struct {
	Namespace   string `json:"namespace"`
	CurrentUser string `json:"current_user"`
	PageOpts
}
