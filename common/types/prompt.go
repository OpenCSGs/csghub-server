package types

import "time"

type PromptReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	CurrentUser string `json:"current_user"`
	Path        string `json:"path"`
}

type Conversation struct {
	Uuid        string   `json:"uuid" binding:"required"`
	Message     string   `json:"message" binding:"required"`
	Temperature *float64 `json:"temperature"`
}

type ConversationReq struct {
	Conversation
	CurrentUser string `json:"current_user"`
}

type ConversationTitle struct {
	Uuid  string `json:"uuid" binding:"required"`
	Title string `json:"title" binding:"required"`
}

type ConversationTitleReq struct {
	ConversationTitle
	CurrentUser string `json:"current_user"`
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMReqBody struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Stream      bool         `json:"stream"`
	Temperature float64      `json:"temperature"`
}

type ConversationMessageReq struct {
	Uuid        string `json:"uuid"`
	Id          int64  `json:"id"`
	CurrentUser string `json:"current_user"`
}

type LLMResponse struct {
	Id                string      `json:"id"`
	Object            string      `json:"object"`
	Created           int64       `json:"created"`
	Model             string      `json:"model"`
	SystemFingerprint string      `json:"system_fingerprint"`
	Choices           []LLMChoice `json:"choices"`
}

type LLMChoice struct {
	Index        int        `json:"index"`
	Delta        LLMMessage `json:"delta"`
	LogProbs     string     `json:"logprobs"`
	FinishReason string     `json:"finish_reason"`
}

type LLMDelta struct {
	Content string `json:"content"`
}

type CreatePromptRepoReq struct {
	CreateRepoReq
}

type UpdatePromptRepoReq struct {
	UpdateRepoReq
}

type PromptRes struct {
	ID            int64                `json:"id,omitempty"`
	Name          string               `json:"name"`
	Nickname      string               `json:"nickname"`
	Description   string               `json:"description"`
	Likes         int64                `json:"likes"`
	Downloads     int64                `json:"downloads"`
	Path          string               `json:"path"`
	RepositoryID  int64                `json:"repository_id"`
	Repository    Repository           `json:"repository"`
	Private       bool                 `json:"private"`
	User          User                 `json:"user"`
	Tags          []RepoTag            `json:"tags"`
	Readme        string               `json:"readme"`
	DefaultBranch string               `json:"default_branch"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	UserLikes     bool                 `json:"user_likes"`
	Source        RepositorySource     `json:"source"`
	SyncStatus    RepositorySyncStatus `json:"sync_status"`
	License       string               `json:"license"`
	CanWrite      bool                 `json:"can_write"`
	CanManage     bool                 `json:"can_manage"`
	Namespace     *Namespace           `json:"namespace"`
}