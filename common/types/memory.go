package types

import "time"

type MemoryProjectRef struct {
	OrgID     string `json:"org_id"`
	ProjectID string `json:"project_id"`
}

type MemoryCapabilities struct {
	SupportsProject     bool `json:"supports_project"`
	SupportsList        bool `json:"supports_list"`
	SupportsMetrics     bool `json:"supports_metrics"`
	SupportsHealthCheck bool `json:"supports_health_check"`
}

type CreateMemoryProjectRequest struct {
	OrgID       string `json:"org_id" binding:"required"`
	ProjectID   string `json:"project_id" binding:"required"`
	Description string `json:"description,omitempty"`
}

type MemoryProjectResponse struct {
	OrgID       string `json:"org_id"`
	ProjectID   string `json:"project_id"`
	Description string `json:"description,omitempty"`
}

type GetMemoryProjectRequest struct {
	OrgID     string `json:"org_id" binding:"required"`
	ProjectID string `json:"project_id" binding:"required"`
}

type DeleteMemoryProjectRequest struct {
	OrgID     string `json:"org_id" binding:"required"`
	ProjectID string `json:"project_id" binding:"required"`
}

type MemoryType string

const (
	MemoryTypeEpisodic MemoryType = "episodic"
	MemoryTypeSemantic MemoryType = "semantic"
)

type MemoryMessage struct {
	UID        string                 `json:"uid,omitempty"`
	Content    string                 `json:"content" binding:"required"`
	Timestamp  time.Time              `json:"timestamp,omitempty"`
	Role       string                 `json:"role,omitempty"`
	Scopes     *MemoryMessageScopes   `json:"scopes,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	MetaData   map[string]any         `json:"meta_data,omitempty"`
	Similarity *float64               `json:"similarity,omitempty"`
	Extra      map[string]interface{} `json:"-"`
}

type MemoryMessageScopes struct {
	AgentID   string `json:"agent_id,omitempty"`
	OrgID     string `json:"org_id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

type AddMemoriesRequest struct {
	AgentID   string          `json:"agent_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	OrgID     string          `json:"org_id,omitempty"`
	ProjectID string          `json:"project_id,omitempty"`
	Types     []MemoryType    `json:"types,omitempty" binding:"dive"`
	Messages  []MemoryMessage `json:"messages" binding:"required,dive"`
}

type MemoryAddResult struct {
	UID string `json:"uid"`
}

type AddMemoriesResponse struct {
	Created []MemoryMessage `json:"created"`
}

type SearchMemoriesRequest struct {
	AgentID       string       `json:"agent_id,omitempty"`
	OrgID         string       `json:"org_id,omitempty"`
	ProjectID     string       `json:"project_id,omitempty"`
	SessionID     string       `json:"session_id,omitempty"`
	ContentQuery  string       `json:"content_query,omitempty"`
	UserID        string       `json:"user_id,omitempty"`
	Role          string       `json:"role,omitempty"`
	TopK          int          `json:"top_k,omitempty" binding:"gte=0"`
	PageSize      int          `json:"page_size,omitempty" binding:"gte=0,required_with=PageNum"`
	PageNum       int          `json:"page_num,omitempty" binding:"gte=0,required_with=PageSize"`
	Filter        string       `json:"filter,omitempty"`
	MinSimilarity *float64     `json:"min_similarity,omitempty" binding:"omitempty,gte=0,lte=1"`
	Types         []MemoryType `json:"types,omitempty" binding:"dive"`
}

type ListMemoriesRequest struct {
	AgentID   string       `json:"agent_id,omitempty"`
	OrgID     string       `json:"org_id,omitempty"`
	ProjectID string       `json:"project_id,omitempty"`
	SessionID string       `json:"session_id,omitempty"`
	UserID    string       `json:"user_id,omitempty"`
	Role      string       `json:"role,omitempty"`
	Types     []MemoryType `json:"types,omitempty" binding:"dive"`
	PageSize  int          `json:"page_size,omitempty" binding:"gte=0,required_with=PageNum"`
	PageNum   int          `json:"page_num,omitempty" binding:"gte=0,required_with=PageSize"`
}

type SearchMemoriesResponse struct {
	Status  int             `json:"status,omitempty"`
	Content []MemoryMessage `json:"content"`
}

type ListMemoriesResponse struct {
	Status  int             `json:"status,omitempty"`
	Content []MemoryMessage `json:"content"`
}

type DeleteMemoriesRequest struct {
	AgentID   string   `json:"agent_id,omitempty"`
	OrgID     string   `json:"org_id,omitempty"`
	ProjectID string   `json:"project_id,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
	UID       string   `json:"uid,omitempty"`
	UIDs      []string `json:"uids,omitempty"`
}

type MemoryHealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}
