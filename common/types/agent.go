package types

import (
	"encoding/json"
	"time"
)

// AgentTemplate represents the template for an agent
type AgentTemplate struct {
	ID          int64           `json:"id"`
	Type        *string         `json:"type" binding:"required"`                 // Possible values: langflow, agno, code, etc.
	UserUUID    *string         `json:"-"`                                       // Will be set from HTTP header using httpbase.GetCurrentUserUUID
	Name        *string         `json:"name" binding:"required,max=255"`         // Agent template name
	Description *string         `json:"description" binding:"omitempty,max=500"` // Agent template description
	Content     *string         `json:"content,omitempty"`                       // Used to store the complete content of the template
	Public      *bool           `json:"public,omitempty"`                        // Whether the template is public
	Metadata    *map[string]any `json:"metadata,omitempty"`                      // Template metadata
	CreatedAt   time.Time       `json:"created_at"`                              // When the template was created
	UpdatedAt   time.Time       `json:"updated_at"`                              // When the template was last updated
}

type AgentTemplateFilter struct {
	Search string
	Type   string
}

// AgentInstance represents an instance created from an agent template
type AgentInstance struct {
	ID          int64           `json:"id"`
	TemplateID  *int64          `json:"template_id" binding:"omitempty,gte=1"` // Associated with the id in the template table
	UserUUID    *string         `json:"-"`                                     // Will be set from HTTP header using httpbase.GetCurrentUserUUID
	Name        *string         `json:"name"`                                  // Instance name
	Description *string         `json:"description" binding:"omitempty"`       // Instance description
	Type        *string         `json:"type"`                                  // Possible values: langflow, agno, code, etc.
	ContentID   *string         `json:"content_id" binding:"omitempty"`        // Used to specify the unique id of the instance resource
	Public      *bool           `json:"public"`                                // Whether the instance is public
	Editable    bool            `json:"editable"`                              // Whether the instance is editable
	IsRunning   bool            `json:"is_running"`                            // Whether the instance is running
	BuiltIn     bool            `json:"built_in"`                              // Whether the instance is built-in
	Metadata    *map[string]any `json:"metadata,omitempty"`                    // Instance metadata
	CreatedAt   time.Time       `json:"created_at"`                            // When the instance was created
	UpdatedAt   time.Time       `json:"updated_at"`                            // When the instance was last updated
}

type AgentType string

const (
	AgentTypeLangflow AgentType = "langflow"
	AgentTypeCode     AgentType = "code"
)

func (t AgentType) String() string {
	return string(t)
}

type AgentInstanceFilter struct {
	Search     string
	Type       string
	TemplateID *int64 `json:"template_id,omitempty"`
	BuiltIn    *bool  `json:"built_in"`
}

type UpdateAgentInstanceRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Metadata    *map[string]any `json:"metadata,omitempty"`
}

type AgentInstanceCreationResult struct {
	ID          string
	Name        string
	Description string
	Metadata    map[string]any // Additional metadata for the agent instance
}

// LangFlowChatRequest represents a chat request to an agent instance
type LangflowChatRequest struct {
	SessionID  *string         `json:"session_id,omitempty"`           // Optional session ID (client-provided)
	InputValue string          `json:"input_value" binding:"required"` // Input value for the agent
	InputType  string          `json:"input_type" binding:"required"`  // Type of input (e.g., "chat")
	OutputType string          `json:"output_type" binding:"required"` // Type of output (e.g., "chat")
	Tweaks     json.RawMessage `json:"tweaks,omitempty"`               // Optional parameter tweaks
}

// AgentChatResponse represents the response from an agent chat
type AgentChatResponse struct {
	SessionID  string `json:"session_id"`  // Session ID used for this conversation
	OutputType string `json:"output_type"` // Output type from the request
	Message    string `json:"message"`     // Agent's response message
	InstanceID int64  `json:"instance_id"` // Agent instance ID
	ContentID  string `json:"content_id"`  // Agent instance content ID
	Type       string `json:"type"`        // Agent instance type
	Timestamp  string `json:"timestamp"`   // When the response was generated
	Sender     string `json:"sender"`      // Which Agent sent the message
}

// AgentChatSession represents a chat session
type AgentInstanceSession struct {
	ID          int64     `json:"id"`
	SessionUUID string    `json:"session_uuid"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // Possible values: langflow, agno, code, etc.
	InstanceID  int64     `json:"instance_id"`
	UserUUID    string    `json:"user_uuid"`
	LastTurn    int64     `json:"last_turn"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AgentInstanceSessionFilter struct {
	InstanceID *int64
}

type CreateAgentInstanceSessionRequest struct {
	SessionUUID *string `json:"session_uuid,omitempty"`
	Name        *string `json:"name,omitempty" binding:"omitempty,max=255"`
	Type        string  `json:"-"` // Possible values: langflow, agno, code, etc.
	InstanceID  *int64  `json:"-"`
	ContentID   *string `json:"-"`
}

type CreateAgentInstanceSessionResponse struct {
	SessionUUID string `json:"session_uuid"`
}

type UpdateAgentInstanceSessionRequest struct {
	Name string `json:"name" binding:"required,max=255"`
}

type RecordAgentInstanceSessionHistoryRequest struct {
	SessionUUID string `json:"session_uuid"`
	Request     bool   `json:"request"`
	Content     string `json:"content"`
}

type CreateSessionHistoryRequest struct {
	SessionUUID string                  `json:"-"`
	Messages    []SessionHistoryMessage `json:"messages" binding:"required"`
}

type SessionHistoryMessage struct {
	Request bool   `json:"request"`           // true: request, false: response
	Content string `json:"content,omitempty"` // message content
}

type CreateSessionHistoryResponse struct {
	MsgUUIDs []string `json:"msg_uuids"`
}

type SessionHistoryMessageType string

const (
	SessionHistoryMessageTypeCreate         SessionHistoryMessageType = "create"
	SessionHistoryMessageTypeUpdateFeedback SessionHistoryMessageType = "update_feedback"
	SessionHistoryMessageTypeRewrite        SessionHistoryMessageType = "rewrite"
)

// SessionHistoryMessageEnvelope is a unified message structure for all session history operations
type SessionHistoryMessageEnvelope struct {
	// Common fields
	MessageType SessionHistoryMessageType `json:"message_type"`
	MsgUUID     string                    `json:"msg_uuid"`
	SessionID   int64                     `json:"session_id"`
	SessionUUID string                    `json:"session_uuid"`
	Request     bool                      `json:"request"` // true: request, false: response

	// Create/Rewrite fields
	Content     string `json:"content,omitempty"`      // message content
	IsRewritten *bool  `json:"is_rewritten,omitempty"` // true: rewritten by user's request

	// UpdateFeedback field
	Feedback *AgentSessionHistoryFeedback `json:"feedback,omitempty"` // feedback: none, like, dislike

	// Rewrite field
	OriginalMsgUUID string `json:"original_msg_uuid,omitempty"` // original message UUID when rewriting
}

// AgentInstanceSessionHistory represents a session history
type AgentInstanceSessionHistory struct {
	ID          int64                       `json:"id"`
	MsgUUID     string                      `json:"msg_uuid"`
	SessionID   int64                       `json:"session_id"`
	SessionUUID string                      `json:"session_uuid"`
	Request     bool                        `json:"request"`
	Content     string                      `json:"content"`
	Feedback    AgentSessionHistoryFeedback `json:"feedback"`
	IsRewritten bool                        `json:"is_rewritten"`
	CreatedAt   time.Time                   `json:"created_at"`
	UpdatedAt   time.Time                   `json:"updated_at"`
}

type AgentInstanceSessionResponse struct {
	SessionUUID string   `json:"session_uuid"`
	Histories   []string `json:"histories"` // list of history contents
}

type AgentSessionHistoryFeedback string

const (
	AgentSessionHistoryFeedbackNone    AgentSessionHistoryFeedback = "none"
	AgentSessionHistoryFeedbackLike    AgentSessionHistoryFeedback = "like"
	AgentSessionHistoryFeedbackDislike AgentSessionHistoryFeedback = "dislike"
)

type FeedbackSessionHistoryRequest struct {
	MsgUUID  string                      `json:"-"`
	Feedback AgentSessionHistoryFeedback `json:"feedback" binding:"required,oneof=none like dislike"`
}

type RewriteSessionHistoryRequest struct {
	OriginalMsgUUID string `json:"-"`
	Content         string `json:"content" binding:"required"`
}

type RewriteSessionHistoryResponse struct {
	MsgUUID string `json:"msg_uuid"`
}

type AgentStreamEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type CodeAgentRequest struct {
	RequestID     string                    `json:"request_id,omitempty"`               // Session ID (client-provided)
	Query         string                    `json:"query" binding:"required"`           // The user's query/question
	MaxLoop       int                       `json:"max_loop" binding:"omitempty,min=1"` // Maximum number of execution loops (default: 1)
	SearchEngines []string                  `json:"search_engines"`                     // List of search engines to use
	Stream        bool                      `json:"stream"`                             // Whether to stream the response
	AgentName     string                    `json:"agent_name" binding:"required"`      // Name of the agent to use
	StreamMode    *StreamMode               `json:"stream_mode,omitempty"`              // Stream configuration
	History       []CodeAgentRequestMessage `json:"history,omitempty"`                  // Conversation history
}

type StreamMode struct {
	Mode  string `json:"mode" binding:"required"` // Stream mode (e.g., "general")
	Token int    `json:"token" binding:"min=1"`   // Token-based streaming interval
	Time  int    `json:"time" binding:"min=1"`    // Time-based streaming interval
}

type CodeAgentRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CodeAgentSyncOperation string

const (
	CodeAgentSyncOperationUpdate CodeAgentSyncOperation = "update"
	CodeAgentSyncOperationDelete CodeAgentSyncOperation = "delete"
)

func (o CodeAgentSyncOperation) String() string {
	return string(o)
}
