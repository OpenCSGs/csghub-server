package types

import "time"

type Credential struct {
	ID             int64          `json:"id"`
	CredentialName string         `json:"credential_name"`
	NamespaceUUID  string         `json:"namespace_uuid"`
	Provider       string         `json:"provider"`
	AuthType       string         `json:"auth_type"`
	Description    string         `json:"description"`
	SecretBackend  string         `json:"secret_backend,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Status         string         `json:"status"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time     `json:"last_used_at,omitempty"`
	ArchivedAt     *time.Time     `json:"archived_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CredentialFilter struct {
	Search string `json:"search"`
}

type CredentialProviderDefinition struct {
	Name             string                    `json:"name"`
	AuthTypes        []string                  `json:"auth_types"`
	CredentialFields []CredentialProviderField `json:"credential_fields,omitempty"`
	MetadataFields   []CredentialProviderField `json:"metadata_fields,omitempty"`
}

type CredentialProviderField struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Secret   bool   `json:"secret,omitempty"`
}

type CreateCredentialRequest struct {
	CredentialName string            `json:"credential_name" binding:"required"`
	Provider       string            `json:"provider" binding:"required"`
	AuthType       string            `json:"auth_type" binding:"required"`
	Description    string            `json:"description,omitempty"`
	Credential     map[string]string `json:"credential" binding:"required"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
}

type UpdateCredentialRequest struct {
	Description *string         `json:"description,omitempty"`
	Metadata    *map[string]any `json:"metadata,omitempty"`
}

type RotateCredentialRequest struct {
	Credential map[string]string `json:"credential" binding:"required"`
}

type CreateTaskCredentialGrantRequest struct {
	TaskID          string   `json:"task_id,omitempty"`
	AgentID         string   `json:"agent_id" binding:"required"`
	CredentialNames []string `json:"credential_names" binding:"required,min=1"`
	DurationSecs    int      `json:"duration_secs,omitempty"`
}

type TaskCredentialGrantResponse struct {
	ID             int64     `json:"id"`
	TaskID         string    `json:"task_id"`
	SessionID      string    `json:"session_id"`
	AgentID        string    `json:"agent_id"`
	CredentialName string    `json:"credential_name,omitempty"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateTaskCredentialGrantResponse struct {
	Grants                 []TaskCredentialGrantResponse `json:"grants"`
	RuntimeCredentialToken string                        `json:"runtime_credential_token"`
	TokenType              string                        `json:"token_type"`
	ExpiresAt              time.Time                     `json:"expires_at"`
}

type RuntimeCredentialResponse struct {
	CredentialName string            `json:"credential_name"`
	Provider       string            `json:"provider"`
	AuthType       string            `json:"auth_type"`
	Credential     map[string]string `json:"credential"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
	ExpiresAt      time.Time         `json:"expires_at"`
}

type RuntimeRefreshRequest struct {
	CredentialName string `json:"credential_name,omitempty"`
}

type RuntimeSessionRevokeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}
