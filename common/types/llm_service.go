package types

import "time"

type RepositoryLite struct {
	ID          int64     `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Nickname    string    `json:"nickname"`
	Description string    `json:"description"`
	Tags        []RepoTag `json:"tags,omitempty"`
}

type LLMConfig struct {
	ID           int64           `json:"id"`
	ModelName    string          `json:"model_name"`
	OfficialName string          `json:"official_name"`
	ApiEndpoint  string          `json:"api_endpoint"`
	AuthHeader   string          `json:"auth_header"`
	Type         int             `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Enabled      bool            `json:"enabled"`
	Provider     string          `json:"provider"`
	Metadata     map[string]any  `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	RepoID       int64           `json:"repo_id"`
	Repo         *RepositoryLite `json:"repo"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type PromptPrefix struct {
	ID   int64  `json:"id"`
	ZH   string `json:"zh"`
	EN   string `json:"en"`
	Kind string `json:"kind"`
}

type SearchLLMConfig struct {
	Keyword string `json:"keyword"` // Search keyword
	Type    *int   `json:"type"`    // Type of search
	Enabled *bool  `json:"enabled"` // Enabled filter
}

type SearchPromptPrefix struct {
	Keyword string `json:"keyword"` // Search keyword
	Kind    string `json:"kind"`    // Kind of prompt prefix
}

type UpdateLLMConfigReq struct {
	ID           int64           `json:"id"`
	ModelName    *string         `json:"model_name"`
	OfficialName *string         `json:"official_name"`
	ApiEndpoint  *string         `json:"api_endpoint"`
	AuthHeader   *string         `json:"auth_header"`
	Type         *int            `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Enabled      *bool           `json:"enabled"`
	Provider     *string         `json:"provider"`
	Metadata     *map[string]any `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	RepoID       *int64          `json:"repo_id"`
}

type UpdatePromptPrefixReq struct {
	ID   int64   `json:"id"`
	ZH   *string `json:"zh"`
	EN   *string `json:"en"`
	Kind *string `json:"kind"`
}

type CreateLLMConfigReq struct {
	ModelName    string         `json:"model_name" binding:"required"`
	OfficialName string         `json:"official_name"`
	ApiEndpoint  string         `json:"api_endpoint" binding:"required"`
	AuthHeader   string         `json:"auth_header"`
	Type         int            `json:"type" binding:"required,oneof=1 2 4 8 16"` // 1: optimization, 2: comparison, 4: summary readme, 8: mcp scan, 16: for aigateway call external llm
	Provider     string         `json:"provider" binding:"required"`
	Enabled      bool           `json:"enabled"`
	Metadata     map[string]any `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	RepoID       *int64         `json:"repo_id"`
}

type CreatePromptPrefixReq struct {
	ZH   string `json:"zh"`
	EN   string `json:"en"`
	Kind string `json:"kind"`
}
