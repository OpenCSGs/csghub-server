package types

import "time"

type LLMConfig struct {
	ID          int64          `json:"id"`
	ModelName   string         `json:"model_name"`
	DisplayName string         `json:"display_name"`
	ApiEndpoint string         `json:"api_endpoint"`
	AuthHeader  string         `json:"auth_header"`
	Type        int            `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Enabled     bool           `json:"enabled"`
	Provider    string         `json:"provider"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
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
}

type SearchPromptPrefix struct {
	Keyword string `json:"keyword"` // Search keyword
	Kind    string `json:"kind"`    // Kind of prompt prefix
}

type UpdateLLMConfigReq struct {
	ID          int64           `json:"id"`
	ModelName   *string         `json:"model_name"`
	DisplayName *string         `json:"display_name"`
	ApiEndpoint *string         `json:"api_endpoint"`
	AuthHeader  *string         `json:"auth_header"`
	Type        *int            `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Enabled     *bool           `json:"enabled"`
	Provider    *string         `json:"provider"`
	Metadata    *map[string]any `json:"metadata"`
}

type UpdatePromptPrefixReq struct {
	ID   int64   `json:"id"`
	ZH   *string `json:"zh"`
	EN   *string `json:"en"`
	Kind *string `json:"kind"`
}

type CreateLLMConfigReq struct {
	ModelName   string         `json:"model_name"`
	DisplayName string         `json:"display_name"`
	ApiEndpoint string         `json:"api_endpoint"`
	AuthHeader  string         `json:"auth_header"`
	Type        int            `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Provider    string         `json:"provider"`
	Enabled     bool           `json:"enabled"`
	Metadata    map[string]any `json:"metadata"`
}
type CreatePromptPrefixReq struct {
	ZH   string `json:"zh"`
	EN   string `json:"en"`
	Kind string `json:"kind"`
}
