package types

import (
	"regexp"
	"time"
)

// mcpServerNameRe enforces the MCP server name format used as a tool prefix.
//
// Rules (derived from MCP SEP-986 tool name spec):
//   - Start with an ASCII letter or digit (not a separator)
//   - Remaining chars: ASCII letters, digits, underscore (_), or hyphen (-)
//   - Max 32 characters — leaves ≥31 chars for the tool name in the
//     prefixed form "{serverName}_{toolName}", keeping the combined
//     name within the 64-char limit mandated by SEP-986
//
// Dots and slashes are intentionally excluded: they carry hierarchical
// meaning in full tool names and could confuse LLM tool lookup.
var mcpServerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,31}$`)

// ValidMCPServerName reports whether s is a valid MCP server name.
func ValidMCPServerName(s string) bool {
	return mcpServerNameRe.MatchString(s)
}

// CreateGatewayMCPServerReq is the request body for creating a gateway MCP server.
type CreateGatewayMCPServerReq struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Protocol    string         `json:"protocol" binding:"required,oneof=streamable sse"`
	URL         string         `json:"url" binding:"required,url"`
	Headers     map[string]any `json:"headers"`
	Env         map[string]any `json:"env"`
	Metadata    map[string]any `json:"metadata"`
}

// GatewayMCPServerReq is the input for updating a gateway MCP server.
// All scalar fields are pointers so callers can distinguish "not provided" from zero value,
// enabling partial updates with the same struct.
type GatewayMCPServerReq struct {
	Name        *string        `json:"name" binding:"omitempty"`
	Description *string        `json:"description"`
	Protocol    *string        `json:"protocol" binding:"omitempty,oneof=streamable sse"`
	URL         *string        `json:"url" binding:"omitempty,url"`
	Headers     map[string]any `json:"headers"`
	Env         map[string]any `json:"env"`
	Metadata    map[string]any `json:"metadata"`
}

// GatewayBackendResponse represents a gateway backend response
type GatewayBackendResponse struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Protocol    string         `json:"protocol"`
	URL         string         `json:"url"`
	Headers     map[string]any `json:"headers,omitempty"`
	Env         map[string]any `json:"env,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// GatewayUserBackendResponse represents a gateway MCP server response (read-only list for users)
type GatewayUserBackendResponse struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Protocol    string         `json:"protocol"`
	URL         string         `json:"url"`
	Headers     map[string]any `json:"headers,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// GatewayMCPServerFilter represents filters for listing gateway MCP servers.
// Fields map to query params in the list API.
type GatewayMCPServerFilter struct {
	// Status filters by cached capability status (e.g. "connected", "error").
	Status *string `json:"status" form:"status"`
	// Search matches server name/description.
	Search string `json:"search" form:"search"`
	// ExactMatch when true disables fuzzy search; search must match name/description exactly.
	ExactMatch bool `json:"exact_match" form:"exact_match"`
}

// GatewayMCPServerResponse is the view of an MCP server including cached capability status.
type GatewayMCPServerResponse struct {
	ID           int64          `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Protocol     string         `json:"protocol"`
	URL          string         `json:"url"`
	Headers      map[string]any `json:"headers,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
	Status       string         `json:"status,omitempty"`
	Error        string         `json:"error,omitempty"`
	RefreshedAt  *time.Time     `json:"refreshed_at,omitempty"`
}

// MCPServerStatus represents the capability cache status of an MCP server.
type MCPServerStatus string

const (
	MCPServerStatusConnected MCPServerStatus = "connected"
	MCPServerStatusError     MCPServerStatus = "error"
)

const (
	CSGHubAccessToken = "CSGHubAccessToken"

	GatewayCapabilityCacheTTL = 1 * time.Hour
)

type GatewayMCPServerInspectTarget struct {
	ServerID   int64          `json:"server_id"`
	Name       string         `json:"name"`
	Protocol   string         `json:"protocol"`
	URL        string         `json:"url"`
	Headers    map[string]any `json:"headers,omitempty"`
	ConfigHash string         `json:"config_hash"`
}
