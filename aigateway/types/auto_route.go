package types

import (
	"context"
	"encoding/json"
)

// AutoRouteMessage is one turn of the visible conversation, flattened to
// plain text.  It is the protocol-neutral form every client protocol
// (chat / responses / messages) is reduced to before the Semantic Router
// ranks candidates for the turn.
type AutoRouteMessage struct {
	Role    string
	Content string
}

// AutoRouteInput describes the current turn to the model ranking service.
// Tools carries the request's tool definitions verbatim so the ranking
// sees the same context the upstream model would.
type AutoRouteInput struct {
	Messages []AutoRouteMessage
	Tools    []json.RawMessage
}

// AutoRouteRequest is one request to resolve the virtual model.  It is a
// struct so the inputs can grow without every caller changing.
type AutoRouteRequest struct {
	// TenantID is the namespace whose visible models may be chosen from.
	TenantID string
	// Input describes the turn to rank.
	Input AutoRouteInput
	// RequiredUpstreamID, when non-zero, pins the conversation to one
	// upstream, so the model that owns it must be reused rather than a
	// new one chosen.
	RequiredUpstreamID int64
}

// AutoRouteDecision records which concrete model automatic routing chose
// and why, so the choice can be logged and replayed later.  BenchmarkID
// is the ranking service's own candidate identifier, which is not a
// usable upstream model name.
type AutoRouteDecision struct {
	ModelID       string
	BenchmarkID   string
	Rank          int
	Score         float64
	IndexVersion  string
	PolicyVersion string
	// Shadowed reports that a real model owns the virtual model's ID, so
	// no ranking took place and ModelID is that model's own ID.  The
	// request is an ordinary one and must be recorded as such.
	Shadowed bool
	// Pinned reports that the conversation was already bound to one
	// upstream, so the model owning it was reused instead of ranked.
	Pinned bool
}

// autoRouteContextKey carries the routing decision on the request context
// so that recording paths far from the Planner — billing in particular —
// can attribute usage to the fact that automatic routing chose the model,
// without every layer in between growing a parameter for it.
type autoRouteContextKey struct{}

// WithAutoRouteDecision returns a context carrying the decision.
func WithAutoRouteDecision(ctx context.Context, decision *AutoRouteDecision) context.Context {
	if decision == nil {
		return ctx
	}
	return context.WithValue(ctx, autoRouteContextKey{}, decision)
}

// AutoRouteDecisionFromContext returns the decision the Planner recorded,
// or nil for a request that named a model directly.
func AutoRouteDecisionFromContext(ctx context.Context) *AutoRouteDecision {
	if ctx == nil {
		return nil
	}
	decision, _ := ctx.Value(autoRouteContextKey{}).(*AutoRouteDecision)
	return decision
}

// AutoRouteContextProvider is implemented by protocol-specific request
// types that can describe themselves to the model ranking service.  It
// mirrors PromptTextProvider: the Planner obtains the turn without
// type-switching on concrete protocol types.
type AutoRouteContextProvider interface {
	AutoRouteContext() AutoRouteInput
}

// AutoRouteContext returns the turn description from the parsed request
// body if it implements AutoRouteContextProvider, otherwise the zero
// value.  A request whose body cannot describe itself simply cannot use
// automatic routing.
func (m *RequestMetadata) AutoRouteContext() AutoRouteInput {
	if m == nil || m.ParsedBody == nil {
		return AutoRouteInput{}
	}
	if p, ok := m.ParsedBody.(AutoRouteContextProvider); ok {
		return p.AutoRouteContext()
	}
	return AutoRouteInput{}
}

// validAutoRouteRoles are the roles the ranking service accepts.  Anything
// else is reported as "user" so an unusual item still contributes its text
// instead of being rejected with a validation error.
var validAutoRouteRoles = map[string]bool{
	"system":    true,
	"developer": true,
	"user":      true,
	"assistant": true,
	"tool":      true,
}

func normalizeAutoRouteRole(role string) string {
	if validAutoRouteRoles[role] {
		return role
	}
	return "user"
}

// AutoRouteContext flattens a Chat Completions request.  Roles map one to
// one; each message contributes the same text the sensitive-content path
// extracts, and the tool definitions are forwarded verbatim.
func (r *ChatCompletionRequest) AutoRouteContext() AutoRouteInput {
	if r == nil {
		return AutoRouteInput{}
	}
	input := AutoRouteInput{Messages: make([]AutoRouteMessage, 0, len(r.Messages))}
	for _, msg := range r.Messages {
		text, ok := chatMessageText(msg)
		if !ok {
			continue
		}
		role := ""
		if r := msg.GetRole(); r != nil {
			role = *r
		}
		input.Messages = append(input.Messages, AutoRouteMessage{
			Role:    normalizeAutoRouteRole(role),
			Content: text,
		})
	}
	for _, tool := range r.Tools {
		if raw, err := json.Marshal(tool); err == nil {
			input.Tools = append(input.Tools, raw)
		}
	}
	return input
}

// AutoRouteContext flattens a Responses request.  Instructions become the
// leading system turn, and each input item becomes one message: items that
// carry an explicit role keep it, while function calls and their outputs
// are attributed to the assistant and the tool respectively.
func (r *ResponsesRequest) AutoRouteContext() AutoRouteInput {
	if r == nil {
		return AutoRouteInput{}
	}
	var input AutoRouteInput
	if instructions := ResponsesInstructionText(r.Instructions); instructions != "" {
		input.Messages = append(input.Messages, AutoRouteMessage{Role: "system", Content: instructions})
	}

	var items []map[string]any
	if len(r.Input) > 0 && json.Unmarshal(r.Input, &items) == nil {
		for _, item := range items {
			role, _ := item["role"].(string)
			switch item["type"] {
			case "function_call":
				role = "assistant"
			case "function_call_output":
				role = "tool"
			}
			input.Messages = append(input.Messages, AutoRouteMessage{
				Role:    normalizeAutoRouteRole(role),
				Content: responsesItemText(item),
			})
		}
	} else if text := ResponsesInputText(r.Input); text != "" {
		input.Messages = append(input.Messages, AutoRouteMessage{Role: "user", Content: text})
	}

	if len(r.Tools) > 0 {
		var tools []json.RawMessage
		if json.Unmarshal(r.Tools, &tools) == nil {
			input.Tools = tools
		}
	}
	return input
}

// AutoRouteContext flattens an Anthropic Messages request.  The system
// prompt becomes the leading system turn and each message contributes the
// same text the sensitive-content path extracts.
func (r *AnthropicMessagesRequest) AutoRouteContext() AutoRouteInput {
	if r == nil {
		return AutoRouteInput{}
	}
	input := AutoRouteInput{Messages: make([]AutoRouteMessage, 0, len(r.Messages)+1)}
	if len(r.System) > 0 {
		if text := AnthropicMessageContentText(r.System); text != "" {
			input.Messages = append(input.Messages, AutoRouteMessage{Role: "system", Content: text})
		}
	}
	for _, msg := range r.Messages {
		input.Messages = append(input.Messages, AutoRouteMessage{
			Role:    normalizeAutoRouteRole(msg.Role),
			Content: AnthropicMessageContentText(msg.Content),
		})
	}
	for _, tool := range r.Tools {
		if raw, err := json.Marshal(tool); err == nil {
			input.Tools = append(input.Tools, raw)
		}
	}
	return input
}
