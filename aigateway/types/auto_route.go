package types

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

// Header names carrying the Semantic Router routing identity.  The router
// owns these IDs (it mints them and validates their format), so they travel
// in HTTP headers only, never in the request or response body.  The gateway
// reads them from the client request, forwards them to the router, and
// echoes the router's response IDs back to the client on the same headers.
const (
	HeaderRouterSessionID = "CSG-Router-Session-Id"
	HeaderRouterTurnID    = "CSG-Router-Turn-Id"
	// HeaderSessionAlias lets a client that already sends X-Session-ID (the
	// gateway's existing session-affinity header) carry the router-assigned
	// session id without adding a second header.  CSG-Router-Session-Id, when
	// present, takes precedence.
	HeaderSessionAlias = "X-Session-ID"
)

// routerIDPatterns mirror the format the Semantic Router enforces on its own
// IDs.  A value that does not match is dropped (treated as "no identity")
// rather than forwarded, because the router would reject it with 422 and
// withdraw the virtual model.  This matters for the X-Session-ID alias:
// existing clients send arbitrary values there for trace/session-affinity,
// and those must not be forwarded as a router session id.
var (
	routerSessionIDPattern = regexp.MustCompile(`^ses_[0-9a-f]{32}$`)
	routerTurnIDPattern    = regexp.MustCompile(`^turn_[0-9a-f]{32}$`)
)

// routingIdentityFromHeaders resolves the router session/turn identity the
// client supplied.  Session falls back to the X-Session-ID alias when the
// explicit CSG-Router-Session-Id header is absent; turn has no alias.  Only
// values matching the router's signed format (ses_<32hex> / turn_<32hex>)
// are returned; anything else is dropped so the router mints fresh IDs
// instead of 422-ing.
func routingIdentityFromHeaders(h http.Header) (session, turn string) {
	session = h.Get(HeaderRouterSessionID)
	if session == "" {
		session = h.Get(HeaderSessionAlias)
	}
	if session != "" && !routerSessionIDPattern.MatchString(session) {
		session = ""
	}
	turn = h.Get(HeaderRouterTurnID)
	if turn != "" && !routerTurnIDPattern.MatchString(turn) {
		turn = ""
	}
	// A turn is meaningful only within a router session.  Dropping an
	// orphaned turn lets the router mint a fresh session instead of
	// rejecting the request with session_id_required_with_turn_id.
	if session == "" {
		turn = ""
	}
	return session, turn
}

// AutoRouteMessage is one turn of the visible conversation, flattened to
// plain text.  It is the protocol-neutral form every client protocol
// (chat / responses / messages) is reduced to before the Semantic Router
// ranks candidates for the turn.
type AutoRouteMessage struct {
	Role    string
	Content string
}

// AutoRouteInput describes the current turn to the model ranking service.
// SystemPrompt is the model-visible system/developer rules, History is the
// conversation before the current user input, and CurrentInput is the
// current user input together with whatever tool messages follow it. Tools
// carries the request's tool definitions verbatim so the ranking sees the
// same context the upstream model would. SessionID and TurnID are the
// routing identity supplied by the client and are forwarded as-is.
type AutoRouteInput struct {
	SystemPrompt string
	History      []AutoRouteMessage
	CurrentInput []AutoRouteMessage
	Tools        []json.RawMessage
	SessionID    string
	TurnID       string
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

// AutoRouteCandidate is one model the ranking service offered that this
// caller can actually use.  BenchmarkID is the ranking service's own
// candidate identifier, which is not a usable upstream model name.
type AutoRouteCandidate struct {
	ModelID     string
	BenchmarkID string
	Rank        int
}

// AutoRouteDecision records which concrete model automatic routing chose
// and why, so the choice can be logged and replayed later.  BenchmarkID
// is the ranking service's own candidate identifier, which is not a
// usable upstream model name.
type AutoRouteDecision struct {
	ModelID      string
	BenchmarkID  string
	Rank         int
	IndexVersion string
	// SessionID and TurnID are the routing identity the router assigned for
	// this turn, echoed back to the client on the response headers so the
	// client can send them back on later turns.  They are empty for a
	// shadowed or pinned decision, where the router was not called.
	SessionID string
	TurnID    string
	// Alternates are the next-ranked models to try, in rank order, when
	// the chosen model fails before any of its response has reached the
	// client.  The caller asked for a model to be picked for it, so a
	// model that turns out not to serve the turn is the router's problem
	// to route around rather than an answer to return.
	//
	// It is empty for a shadowed or pinned decision, where the model was
	// not ranked at all and there is nothing to fall back to.
	Alternates []AutoRouteCandidate
	// Shadowed reports that a real model owns the virtual model's ID, so
	// no ranking took place and ModelID is that model's own ID.  The
	// request is an ordinary one and must be recorded as such.
	Shadowed bool
	// Pinned reports that the conversation was already bound to one
	// upstream, so the model owning it was reused instead of ranked.
	Pinned bool
	// Fallbacks counts how many ranked models were tried and failed before
	// the one now named here.  It is 0 for the common case of a
	// first-choice model that served the turn, so a billing record that
	// carries it is exactly one produced by a fallback.
	Fallbacks int
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
// automatic routing.  The router session/turn identity is overlaid from
// the request headers (CSG-Router-Session-Id / CSG-Router-Turn-Id, with
// X-Session-ID as a session alias), not the body.
func (m *RequestMetadata) AutoRouteContext() AutoRouteInput {
	if m == nil || m.ParsedBody == nil {
		return AutoRouteInput{}
	}
	var input AutoRouteInput
	if p, ok := m.ParsedBody.(AutoRouteContextProvider); ok {
		input = p.AutoRouteContext()
	}
	input.SessionID, input.TurnID = routingIdentityFromHeaders(m.Headers)
	return input
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

// newAutoRouteInput assigns the flattened messages to the ranking
// service's three slots: system/developer text becomes the system prompt,
// everything before the last user message becomes history, and the last
// user message with whatever follows it becomes the current input.  The
// routing identity is not part of the body; it is overlaid by
// RequestMetadata.AutoRouteContext from the request headers.
func newAutoRouteInput(messages []AutoRouteMessage) AutoRouteInput {
	systemPrompt, history, currentInput := splitAutoRouteMessages(messages)
	return AutoRouteInput{
		SystemPrompt: systemPrompt,
		History:      history,
		CurrentInput: currentInput,
	}
}

// splitAutoRouteMessages splits a flattened message list into the semantic
// router's three slots.  system/developer messages are joined into the
// system prompt, messages before the last user message become history, and
// the last user message with whatever follows it becomes the current input.
// Messages with empty content are dropped, because the ranking service
// rejects them.  The last user message is located among the non-empty
// messages, so a trailing empty user turn cannot become the current input
// and leave it empty.
func splitAutoRouteMessages(messages []AutoRouteMessage) (systemPrompt string, history, currentInput []AutoRouteMessage) {
	lastUser := -1
	var sysParts []string
	for i, m := range messages {
		switch m.Role {
		case "system", "developer":
			if text := strings.TrimSpace(m.Content); text != "" {
				sysParts = append(sysParts, m.Content)
			}
		case "user":
			if strings.TrimSpace(m.Content) != "" {
				lastUser = i
			}
		}
	}
	systemPrompt = strings.Join(sysParts, "\n")

	for i, m := range messages {
		role := m.Role
		if role == "system" || role == "developer" {
			continue
		}
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		switch {
		case lastUser < 0 || i < lastUser:
			history = append(history, m)
		case role == "user" || role == "tool":
			currentInput = append(currentInput, m)
		default:
			history = append(history, m)
		}
	}
	return systemPrompt, history, currentInput
}

// AutoRouteContext flattens a Chat Completions request.  Roles map one to
// one; each message contributes the same text the sensitive-content path
// extracts, and the tool definitions are forwarded verbatim.
func (r *ChatCompletionRequest) AutoRouteContext() AutoRouteInput {
	if r == nil {
		return AutoRouteInput{}
	}
	messages := make([]AutoRouteMessage, 0, len(r.Messages))
	for _, msg := range r.Messages {
		text, ok := chatMessageText(msg)
		if !ok {
			continue
		}
		role := ""
		if r := msg.GetRole(); r != nil {
			role = *r
		}
		messages = append(messages, AutoRouteMessage{
			Role:    normalizeAutoRouteRole(role),
			Content: text,
		})
	}
	input := newAutoRouteInput(messages)
	for _, tool := range r.Tools {
		if raw, err := json.Marshal(tool); err == nil {
			input.Tools = append(input.Tools, raw)
		}
	}
	return input
}

// AutoRouteContext flattens a Responses request.  Instructions become the
// system prompt, and each input item becomes one message: items that carry
// an explicit role keep it, while function calls and their outputs are
// attributed to the assistant and the tool respectively.
func (r *ResponsesRequest) AutoRouteContext() AutoRouteInput {
	if r == nil {
		return AutoRouteInput{}
	}
	var messages []AutoRouteMessage
	if instructions := ResponsesInstructionText(r.Instructions); instructions != "" {
		messages = append(messages, AutoRouteMessage{Role: "system", Content: instructions})
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
			messages = append(messages, AutoRouteMessage{
				Role:    normalizeAutoRouteRole(role),
				Content: responsesItemText(item),
			})
		}
	} else if text := ResponsesInputText(r.Input); text != "" {
		messages = append(messages, AutoRouteMessage{Role: "user", Content: text})
	}

	input := newAutoRouteInput(messages)
	if len(r.Tools) > 0 {
		var tools []json.RawMessage
		if json.Unmarshal(r.Tools, &tools) == nil {
			input.Tools = tools
		}
	}
	return input
}

// AutoRouteContext flattens an Anthropic Messages request.  The system
// prompt becomes the system prompt and each message contributes the same
// text the sensitive-content path extracts.
func (r *AnthropicMessagesRequest) AutoRouteContext() AutoRouteInput {
	if r == nil {
		return AutoRouteInput{}
	}
	messages := make([]AutoRouteMessage, 0, len(r.Messages)+1)
	if len(r.System) > 0 {
		if text := AnthropicMessageContentText(r.System); text != "" {
			messages = append(messages, AutoRouteMessage{Role: "system", Content: text})
		}
	}
	for _, msg := range r.Messages {
		messages = append(messages, AutoRouteMessage{
			Role:    normalizeAutoRouteRole(msg.Role),
			Content: AnthropicMessageContentText(msg.Content),
		})
	}
	input := newAutoRouteInput(messages)
	for _, tool := range r.Tools {
		if raw, err := json.Marshal(tool); err == nil {
			input.Tools = append(input.Tools, raw)
		}
	}
	return input
}
