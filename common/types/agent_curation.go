package types

import "time"

// AgentCurationEntityType identifies the agent-domain resource a curation row
// applies to in the operations-management catalog.
type AgentCurationEntityType string

const (
	AgentCurationEntityTypeAgentInstance      AgentCurationEntityType = "agent_instance"
	AgentCurationEntityTypeAgentTemplate      AgentCurationEntityType = "agent_template"
	AgentCurationEntityTypeAgentSkill         AgentCurationEntityType = "agent_skill"
	AgentCurationEntityTypeAgentMCPServer     AgentCurationEntityType = "agent_mcp_server"
	AgentCurationEntityTypeAgentKnowledgeBase AgentCurationEntityType = "agent_knowledge_base"
	AgentCurationEntityTypeAgentPrompt        AgentCurationEntityType = "agent_prompt"
	// Reserved: agent_quickstart_template.
)

// AgentCurationOrderItem is the minimal per-resource state the move/position
// reorder algorithm needs from the canonical visible ordering.
type AgentCurationOrderItem struct {
	ID          int64
	Recommended bool
}

// AgentCurationMoveDirection is the relative move operation for a curated resource.
type AgentCurationMoveDirection string

const (
	AgentCurationMoveUp     AgentCurationMoveDirection = "up"
	AgentCurationMoveDown   AgentCurationMoveDirection = "down"
	AgentCurationMoveBottom AgentCurationMoveDirection = "bottom"
)

// MoveAgentResourceRequest moves a resource one slot up/down or to the bottom
// of its recommended group.
type MoveAgentResourceRequest struct {
	Direction AgentCurationMoveDirection `json:"direction" binding:"required,oneof=up down bottom"`
}

// SetAgentResourcePositionRequest moves a resource to a 1-based position within
// its recommended group; out-of-range positions clamp to the group edge.
type SetAgentResourcePositionRequest struct {
	Position int `json:"position" binding:"required,gte=1"`
}

// SetAgentResourceRecommendRequest toggles the recommended badge.
type SetAgentResourceRecommendRequest struct {
	Recommended bool `json:"recommended"`
}

// SetAgentResourceUnlistRequest toggles hiding the resource from public lists.
type SetAgentResourceUnlistRequest struct {
	Unlisted bool `json:"unlisted"`
}

// ResetAgentCurationRequest deletes curation rows; empty entity type resets all.
type ResetAgentCurationRequest struct {
	EntityType string `json:"entity_type"`
}

// AdminAgentResourceCurationItem is the admin-list view of a public agent
// resource together with its curation state. Fields not applicable to a
// resource type retain their zero values.
type AdminAgentResourceCurationItem struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Nickname      string    `json:"nickname"`
	Description   string    `json:"description"`
	Type          string    `json:"type"`
	UserUUID      string    `json:"user_uuid"`
	NamespacePath string    `json:"namespace_path"`
	BuiltIn       bool      `json:"built_in"`
	Recommended   bool      `json:"recommended"`
	Weight        int64     `json:"weight"`
	Unlisted      bool      `json:"unlisted"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
