package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"

	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func validateResponsesAdapterRequest(req *types.ResponsesRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	if req.PreviousResponseID != "" {
		return unsupportedResponsesFeature("previous_response_id")
	}
	if isTrue(req.Store) {
		return unsupportedResponsesFeature("store")
	}
	if isTrue(req.Background) {
		return unsupportedResponsesFeature("background")
	}
	if len(req.Conversation) > 0 {
		return unsupportedResponsesFeature("conversation")
	}
	if len(req.Prompt) > 0 {
		return unsupportedResponsesFeature("prompt")
	}
	if req.MaxToolCalls != nil {
		return unsupportedResponsesFeature("max_tool_calls")
	}
	return nil
}

func unsupportedResponsesFeature(field string) error {
	return fmt.Errorf("unsupported_feature:%s", field)
}

func isTrue(v *bool) bool {
	return v != nil && *v
}

// normalizeChatRole maps OpenAI Responses-style roles to chat completions roles.
// The developer role is used by OpenAI Responses-style inputs. Many Chat
// Completions-compatible upstreams only accept system/user/assistant/tool,
// so developer is downgraded to system.
func normalizeChatRole(role string) string {
	switch role {
	case "":
		return "user"
	case "developer":
		return "system"
	default:
		return role
	}
}

func responsesToChatRequest(ctx context.Context, req *types.ResponsesRequest, modelName string, upstreamMetadata *commontypes.UpstreamMetadata) (*types.ChatCompletionRequest, error) {
	chatReq, _, err := responsesToChatRequestResolved(ctx, req, modelName, upstreamMetadata)
	return chatReq, err
}

// responsesToChatRequestResolved converts a Responses request into a chat
// completions request and returns the request-scoped tool alias resolver used
// to build it. The caller feeds the resolver to the adapter response writers so
// they can restore original tool names and namespaces without parsing the tool
// definitions a second time.
func responsesToChatRequestResolved(ctx context.Context, req *types.ResponsesRequest, modelName string, upstreamMetadata *commontypes.UpstreamMetadata) (*types.ChatCompletionRequest, *responsesToolAliases, error) {
	toolAliases := newResponsesToolAliases()
	chatReq := &types.ChatCompletionRequest{
		Model:       modelName,
		Stream:      req.Stream,
		Temperature: floatPtrValue(req.Temperature),
		TopP:        floatPtrValue(req.TopP),
	}
	// Convert tools before messages so history remapping can consult the aliases.
	if len(req.Tools) > 0 {
		chatTools, err := responsesToolsToChatTools(ctx, req.Tools, modelName, toolAliases)
		if err != nil {
			return nil, nil, err
		}
		if len(chatTools) > 0 {
			if err := json.Unmarshal(chatTools, &chatReq.Tools); err != nil {
				return nil, nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
			}
		}
	}
	messages, err := responsesInputToChatMessages(ctx, req, toolAliases)
	if err != nil {
		return nil, nil, err
	}
	rawMessages, err := json.Marshal(messages)
	if err != nil {
		return nil, nil, err
	}
	var sdkMessages []openai.ChatCompletionMessageParamUnion
	if err := json.Unmarshal(rawMessages, &sdkMessages); err != nil {
		return nil, nil, fmt.Errorf("convert responses input to chat messages: %w", err)
	}
	chatReq.Messages = sdkMessages
	mergeChatRawJSONRaw(chatReq, "messages", rawMessages)
	if req.MaxOutputTokens != nil {
		chatReq.MaxTokens = *req.MaxOutputTokens
	}
	if len(req.ToolChoice) > 0 {
		if !json.Valid(req.ToolChoice) {
			return nil, nil, fmt.Errorf("invalid responses tool_choice")
		}
		chatToolChoice, err := responsesToolChoiceToChatToolChoice(ctx, req.ToolChoice, toolAliases, modelName)
		if err != nil {
			return nil, nil, err
		}
		if len(chatToolChoice) > 0 {
			if err := json.Unmarshal(chatToolChoice, &chatReq.ToolChoice); err != nil {
				return nil, nil, fmt.Errorf("convert responses tool_choice to chat tool_choice: %w", err)
			}
			mergeChatRawJSONRaw(chatReq, "tool_choice", chatToolChoice)
		}
	}
	parallel := true
	if req.ParallelToolCalls != nil {
		parallel = *req.ParallelToolCalls
	}
	mergeChatRawJSON(chatReq, "parallel_tool_calls", parallel)
	if len(req.Text) > 0 {
		var textObj map[string]json.RawMessage
		if err := json.Unmarshal(req.Text, &textObj); err == nil {
			if format, ok := textObj["format"]; ok {
				mergeChatRawJSONRaw(chatReq, "response_format", format)
			}
		}
	}
	if len(req.Reasoning) > 0 {
		cfg := loadReasoningRequestConfig(upstreamMetadata)
		if err := applyAdapterReasoningRequest(ctx, chatReq, cfg, req.Reasoning); err != nil {
			return nil, nil, err
		}
	}
	return chatReq, toolAliases, nil
}

func mergeChatRawJSON(chatReq *types.ChatCompletionRequest, key string, value any) {
	rawValue, err := json.Marshal(value)
	if err != nil {
		return
	}
	mergeChatRawJSONRaw(chatReq, key, rawValue)
}

func mergeChatRawJSONRaw(chatReq *types.ChatCompletionRequest, key string, value json.RawMessage) {
	if chatReq == nil || key == "" {
		return
	}
	raw := map[string]json.RawMessage{}
	if len(chatReq.RawJSON) > 0 {
		_ = json.Unmarshal(chatReq.RawJSON, &raw)
	}
	raw[key] = value
	chatReq.RawJSON, _ = json.Marshal(raw)
}

var knownReasoningEfforts = map[string]string{
	"none":    "none",
	"low":     "low",
	"medium":  "medium",
	"high":    "high",
	"minimal": "minimal",
	// xhigh is a Responses API effort value not widely supported by chat upstreams;
	// normalize to max, the highest effort level those upstreams typically accept.
	"xhigh": "max",
	"max":   "max",
}

type adapterReasoningRequestConfig struct {
	Enabled      bool            `json:"enabled"`
	EffortField  string          `json:"effort_field"`
	EnableExtra  json.RawMessage `json:"enable_extra"`
	DisableExtra json.RawMessage `json:"disable_extra"`
}

func loadReasoningRequestConfig(metadata *commontypes.UpstreamMetadata) *adapterReasoningRequestConfig {
	if metadata == nil || metadata.ResponsesChatAdapter == nil {
		return nil
	}
	rr := metadata.ResponsesChatAdapter.ReasoningRequest
	if rr == nil {
		return nil
	}
	cfg := &adapterReasoningRequestConfig{
		Enabled:     rr.Enabled,
		EffortField: rr.EffortField,
	}
	if rr.EnableExtra != nil {
		raw, err := json.Marshal(rr.EnableExtra)
		if err != nil {
			return nil
		}
		cfg.EnableExtra = raw
	}
	if rr.DisableExtra != nil {
		raw, err := json.Marshal(rr.DisableExtra)
		if err != nil {
			return nil
		}
		cfg.DisableExtra = raw
	}
	return cfg
}

func applyAdapterReasoningRequest(ctx context.Context, chatReq *types.ChatCompletionRequest, cfg *adapterReasoningRequestConfig, rawReasoning json.RawMessage) error {
	if cfg == nil {
		return nil
	}
	effort := parseReasoningEffort(rawReasoning)
	if effort == "" {
		return nil
	}
	normalized, known := normalizeEffort(effort)
	if !known {
		slog.WarnContext(ctx, "reject unknown reasoning effort for chat adapter",
			slog.String("api", "/v1/responses"),
			slog.String("adapter", "chat_completions"),
			slog.String("effort", effort))
		return fmt.Errorf("invalid reasoning effort: %q", effort)
	}
	if !cfg.Enabled {
		if normalized == "none" {
			slog.InfoContext(ctx, "drop reasoning request for disabled upstream",
				slog.String("api", "/v1/responses"),
				slog.String("adapter", "chat_completions"),
				slog.String("effort", normalized))
			return nil
		}
		slog.WarnContext(ctx, "reject reasoning request for disabled upstream",
			slog.String("api", "/v1/responses"),
			slog.String("adapter", "chat_completions"),
			slog.String("effort", normalized))
		return unsupportedResponsesFeature("reasoning")
	}
	if normalized == "none" {
		if err := mergeChatRawJSONObject(chatReq, cfg.DisableExtra); err != nil {
			return err
		}
		slog.InfoContext(ctx, "merge chat adapter reasoning disable_extra",
			slog.String("api", "/v1/responses"),
			slog.String("adapter", "chat_completions"),
			slog.String("effort", normalized))
		return nil
	}
	if cfg.EffortField != "" {
		mergeChatRawJSON(chatReq, cfg.EffortField, normalized)
	}
	if err := mergeChatRawJSONObject(chatReq, cfg.EnableExtra); err != nil {
		return err
	}
	slog.InfoContext(ctx, "merge chat adapter reasoning enable fields",
		slog.String("api", "/v1/responses"),
		slog.String("adapter", "chat_completions"),
		slog.String("effort", normalized),
		slog.String("effort_field", cfg.EffortField))
	return nil
}

func parseReasoningEffort(rawReasoning json.RawMessage) string {
	if len(rawReasoning) == 0 {
		return ""
	}
	var reasoning struct {
		Effort string `json:"effort"`
	}
	if err := json.Unmarshal(rawReasoning, &reasoning); err != nil {
		return ""
	}
	return strings.TrimSpace(reasoning.Effort)
}

func normalizeEffort(effort string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	if normalized == "" {
		return "", false
	}
	mapped, ok := knownReasoningEfforts[normalized]
	if !ok {
		return "", false
	}
	return mapped, true
}

func mergeChatRawJSONObject(chatReq *types.ChatCompletionRequest, extra json.RawMessage) error {
	if len(extra) == 0 || string(extra) == "null" {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(extra, &obj); err != nil {
		return fmt.Errorf("invalid reasoning extra json: %w", err)
	}
	for key, value := range obj {
		mergeChatRawJSONRaw(chatReq, key, value)
	}
	return nil
}

// responsesToolIdentity is the original, client-visible identity of a function
// tool. An empty Namespace means the function was declared at the top level.
type responsesToolIdentity struct {
	Name      string
	Namespace string
}

// responsesToolAliases records the mapping between the Responses function tools
// exposed by a request and the tool names sent to the chat completions upstream.
// Identities whose original function name is unambiguous keep that name; only
// colliding identities (the same name in more than one namespace, or a top-level
// function sharing the name) are rewritten to a deterministic alias. Output
// writers use identity() to restore the original name and namespace.
type responsesToolAliases struct {
	byUpstreamName          map[string]responsesToolIdentity
	upstreamNamesByToolName map[string][]string
	identityToUpstream      map[responsesToolIdentity]string
	identities              map[responsesToolIdentity]struct{}
}

func newResponsesToolAliases() *responsesToolAliases {
	return &responsesToolAliases{
		byUpstreamName:          map[string]responsesToolIdentity{},
		upstreamNamesByToolName: map[string][]string{},
		identityToUpstream:      map[responsesToolIdentity]string{},
		identities:              map[responsesToolIdentity]struct{}{},
	}
}

func (a *responsesToolAliases) register(identity responsesToolIdentity, upstreamName string) error {
	if _, ok := a.identities[identity]; ok {
		if identity.Namespace != "" {
			return fmt.Errorf("duplicated tool name within namespace %s: %s", identity.Namespace, identity.Name)
		}
		return fmt.Errorf("duplicated function tool name: %s", identity.Name)
	}
	if existing, ok := a.byUpstreamName[upstreamName]; ok && existing != identity {
		return fmt.Errorf("tool alias collision: %s", upstreamName)
	}
	a.identities[identity] = struct{}{}
	a.byUpstreamName[upstreamName] = identity
	a.identityToUpstream[identity] = upstreamName
	a.upstreamNamesByToolName[identity.Name] = append(a.upstreamNamesByToolName[identity.Name], upstreamName)
	return nil
}

// identity resolves an upstream chat tool name back to its original Responses
// identity. Unknown names and top-level functions resolve to themselves, so
// output writers pass them through unchanged.
func (a *responsesToolAliases) identity(upstreamName string) responsesToolIdentity {
	if a != nil {
		if identity, ok := a.byUpstreamName[upstreamName]; ok {
			return identity
		}
	}
	return responsesToolIdentity{Name: upstreamName}
}

// chatName returns the single upstream name assigned to an original function
// name and whether that name differs from the original (i.e. was aliased). It
// returns ok=false so callers leave the name untouched when the function is
// unknown or shared by more than one identity.
func (a *responsesToolAliases) chatName(functionName string) (string, bool) {
	if a == nil {
		return "", false
	}
	names := a.upstreamNamesByToolName[functionName]
	if len(names) == 1 {
		return names[0], names[0] != functionName
	}
	return "", false
}

// chatNameForIdentity returns the upstream name assigned to an explicit
// namespace+function identity, used when a historical tool call records its
// namespace and the original function name alone is ambiguous.
func (a *responsesToolAliases) chatNameForIdentity(identity responsesToolIdentity) (string, bool) {
	if a == nil {
		return "", false
	}
	name, ok := a.identityToUpstream[identity]
	return name, ok
}

func responsesNamespacedToolAlias(namespaceName, functionName string) string {
	const (
		maxNameLength = 64
		hashLength    = 32
		separator     = "__"
	)
	prefix := responsesSanitizeToolName(namespaceName) + separator + responsesSanitizeToolName(functionName)
	maxPrefixLength := maxNameLength - len(separator) - hashLength
	if len(prefix) > maxPrefixLength {
		prefix = prefix[:maxPrefixLength]
	}
	sum := sha256.Sum256([]byte(namespaceName + "\x00" + functionName))
	return fmt.Sprintf("%s%s%x", prefix, separator, sum[:hashLength/2])
}

func responsesSanitizeToolName(name string) string {
	var result strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '_', char == '-':
			result.WriteRune(char)
		default:
			result.WriteByte('_')
		}
	}
	return result.String()
}

type responsesFunctionToolEntry struct {
	chatTool map[string]json.RawMessage
	identity responsesToolIdentity
}

// responsesToolsToChatTools flattens Responses tools into chat completions
// function tools. It collects every function identity first so it can tell
// whether an original name is ambiguous across the whole request, then assigns
// chat tool names: unambiguous identities keep their name, and only colliding
// namespaced identities are rewritten to a deterministic alias. A kept original
// name that is claimed as another identity's alias is itself rewritten, so an
// assigned upstream name never collides with a client-declared function name.
func responsesToolsToChatTools(ctx context.Context, raw json.RawMessage, modelName string, toolAliases *responsesToolAliases) (json.RawMessage, error) {
	entries, err := responsesCollectFunctionTools(ctx, raw, modelName)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	nameCount := make(map[string]int, len(entries))
	for _, entry := range entries {
		if entry.identity.Name != "" {
			nameCount[entry.identity.Name]++
		}
	}
	// Assign upstream chat names before registering anything so a generated alias
	// can never shadow an original name that another identity keeps. Aliases are
	// drawn from the same charset as client function names, so a client may
	// legitimately declare a top-level function whose name equals the alias of a
	// colliding namespaced identity. Such an occupant must yield its original name
	// (it is rewritten to its own alias) rather than rejecting the request.
	upstreamName := make([]string, len(entries))
	aliased := make([]bool, len(entries))
	claimed := make(map[string]struct{}, len(entries))
	for i, entry := range entries {
		upstreamName[i] = entry.identity.Name
		if entry.identity.Namespace != "" && entry.identity.Name != "" && nameCount[entry.identity.Name] > 1 {
			aliased[i] = true
			upstreamName[i] = responsesNamespacedToolAlias(entry.identity.Namespace, entry.identity.Name)
			claimed[upstreamName[i]] = struct{}{}
		}
	}
	// The alias set only grows and each pass rewrites a distinct previously-kept
	// identity, so this terminates; the least fixpoint is independent of tool order.
	for {
		changed := false
		for i, entry := range entries {
			if aliased[i] || entry.identity.Name == "" {
				continue
			}
			if _, ok := claimed[entry.identity.Name]; !ok {
				continue
			}
			aliased[i] = true
			upstreamName[i] = responsesNamespacedToolAlias(entry.identity.Namespace, entry.identity.Name)
			claimed[upstreamName[i]] = struct{}{}
			changed = true
		}
		if !changed {
			break
		}
	}
	chatTools := make([]map[string]json.RawMessage, 0, len(entries))
	for i, entry := range entries {
		name := upstreamName[i]
		if name == "" {
			chatTools = append(chatTools, entry.chatTool)
			continue
		}
		if err := toolAliases.register(entry.identity, name); err != nil {
			return nil, err
		}
		if name != entry.identity.Name {
			if err := responsesSetChatToolName(entry.chatTool, name); err != nil {
				return nil, err
			}
		}
		chatTools = append(chatTools, entry.chatTool)
	}
	data, err := json.Marshal(chatTools)
	if err != nil {
		return nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
	}
	return data, nil
}

func responsesCollectFunctionTools(ctx context.Context, raw json.RawMessage, modelName string) ([]responsesFunctionToolEntry, error) {
	var entries []responsesFunctionToolEntry
	for _, tool := range splitRawJSONArray(raw) {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(tool, &obj); err != nil {
			return nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
		}
		var toolType string
		_ = json.Unmarshal(obj["type"], &toolType)
		if toolType == "namespace" {
			namespaceEntries, err := responsesCollectNamespaceFunctionTools(ctx, obj, modelName)
			if err != nil {
				return nil, err
			}
			entries = append(entries, namespaceEntries...)
			if len(namespaceEntries) == 0 {
				slog.InfoContext(ctx, "drop empty responses namespace tool for chat adapter",
					slog.String("api", "/v1/responses"),
					slog.String("adapter", "chat_completions"),
					slog.String("model", modelName),
					slog.String("dropped_tool_type", toolType))
			}
			continue
		}
		if toolType != "" && toolType != "function" {
			// Chat completions APIs only support function tools.
			// Drop unsupported tool types to avoid upstream errors.
			slog.InfoContext(ctx, "drop unsupported responses tool for chat adapter",
				slog.String("api", "/v1/responses"),
				slog.String("adapter", "chat_completions"),
				slog.String("model", modelName),
				slog.String("dropped_tool_type", toolType))
			continue
		}
		entry, err := responsesFunctionToolEntryFromObject(obj, "")
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func responsesCollectNamespaceFunctionTools(ctx context.Context, obj map[string]json.RawMessage, modelName string) ([]responsesFunctionToolEntry, error) {
	rawChildren := responsesNamespaceToolChildren(obj)
	namespaceName := responsesNamespaceName(obj)
	var entries []responsesFunctionToolEntry
	if len(rawChildren) == 0 && responsesLooksLikeFunctionTool(obj) {
		entry, err := responsesFunctionToolEntryFromObject(obj, namespaceName)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	for _, rawChild := range rawChildren {
		var child map[string]json.RawMessage
		if err := json.Unmarshal(rawChild, &child); err != nil {
			return nil, fmt.Errorf("convert responses namespace tool: %w", err)
		}
		var childType string
		_ = json.Unmarshal(child["type"], &childType)
		if childType != "" && childType != "function" {
			slog.InfoContext(ctx, "drop unsupported responses namespace child tool for chat adapter",
				slog.String("api", "/v1/responses"),
				slog.String("adapter", "chat_completions"),
				slog.String("model", modelName),
				slog.String("dropped_tool_type", childType))
			continue
		}
		entry, err := responsesFunctionToolEntryFromObject(child, namespaceName)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func responsesFunctionToolEntryFromObject(obj map[string]json.RawMessage, namespaceName string) (responsesFunctionToolEntry, error) {
	chatTool, functionName, err := responsesFunctionToolToChatTool(obj)
	if err != nil {
		return responsesFunctionToolEntry{}, err
	}
	return responsesFunctionToolEntry{
		chatTool: chatTool,
		identity: responsesToolIdentity{Name: functionName, Namespace: namespaceName},
	}, nil
}

func responsesSetChatToolName(chatTool map[string]json.RawMessage, upstreamName string) error {
	if upstreamName == "" {
		return nil
	}
	var function map[string]json.RawMessage
	if err := json.Unmarshal(chatTool["function"], &function); err != nil {
		return fmt.Errorf("convert responses function tool: %w", err)
	}
	function["name"], _ = json.Marshal(upstreamName)
	functionRaw, err := json.Marshal(function)
	if err != nil {
		return fmt.Errorf("convert responses function tool: %w", err)
	}
	chatTool["function"] = functionRaw
	return nil
}

func responsesNamespaceName(obj map[string]json.RawMessage) string {
	var name string
	_ = json.Unmarshal(obj["name"], &name)
	if name != "" {
		return name
	}
	var namespace map[string]json.RawMessage
	if err := json.Unmarshal(obj["namespace"], &namespace); err != nil {
		return ""
	}
	_ = json.Unmarshal(namespace["name"], &name)
	return name
}

func responsesNamespaceToolChildren(obj map[string]json.RawMessage) []json.RawMessage {
	var children []json.RawMessage
	for _, key := range []string{"tools", "functions"} {
		children = append(children, splitRawJSONArray(obj[key])...)
	}
	var namespace map[string]json.RawMessage
	if err := json.Unmarshal(obj["namespace"], &namespace); err == nil {
		for _, key := range []string{"tools", "functions"} {
			children = append(children, splitRawJSONArray(namespace[key])...)
		}
	}
	return children
}

func responsesLooksLikeFunctionTool(obj map[string]json.RawMessage) bool {
	if _, ok := obj["function"]; ok {
		return true
	}
	if _, ok := obj["name"]; !ok {
		return false
	}
	for _, key := range []string{"parameters", "description", "strict"} {
		if _, ok := obj[key]; ok {
			return true
		}
	}
	return false
}

func responsesFunctionToolName(obj map[string]json.RawMessage) string {
	function := map[string]json.RawMessage{}
	if rawFunction, ok := obj["function"]; ok {
		_ = json.Unmarshal(rawFunction, &function)
	} else {
		function = obj
	}
	var functionName string
	_ = json.Unmarshal(function["name"], &functionName)
	return functionName
}

func responsesFunctionToolToChatTool(obj map[string]json.RawMessage) (map[string]json.RawMessage, string, error) {
	function := map[string]json.RawMessage{}
	if rawFunction, ok := obj["function"]; ok {
		if err := json.Unmarshal(rawFunction, &function); err != nil || function == nil {
			return nil, "", fmt.Errorf("convert responses function tool: function must be an object")
		}
	} else {
		for _, key := range []string{"name", "description", "parameters", "strict"} {
			if value, ok := obj[key]; ok {
				function[key] = value
			}
		}
	}
	functionRaw, err := json.Marshal(function)
	if err != nil {
		return nil, "", fmt.Errorf("convert responses function tool: %w", err)
	}
	functionName := responsesFunctionToolName(obj)
	chatTool := map[string]json.RawMessage{
		"type":     json.RawMessage(`"function"`),
		"function": functionRaw,
	}
	return chatTool, functionName, nil
}

func responsesToolChoiceToChatToolChoice(ctx context.Context, raw json.RawMessage, toolAliases *responsesToolAliases, modelName string) (json.RawMessage, error) {
	var choice string
	if err := json.Unmarshal(raw, &choice); err == nil {
		if choice == "required" && len(toolAliases.byUpstreamName) == 0 {
			return nil, nil
		}
		return raw, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		slog.WarnContext(ctx, "drop unsupported responses tool_choice for chat adapter",
			slog.String("api", "/v1/responses"),
			slog.String("adapter", "chat_completions"),
			slog.String("model", modelName),
			slog.Any("error", err))
		return nil, nil
	}
	var toolType string
	_ = json.Unmarshal(obj["type"], &toolType)
	if toolType != "function" {
		return nil, nil
	}
	var function map[string]json.RawMessage
	if err := json.Unmarshal(obj["function"], &function); err != nil {
		return nil, nil
	}
	var functionName string
	_ = json.Unmarshal(function["name"], &functionName)
	upstreamNames := toolAliases.upstreamNamesByToolName[functionName]
	if len(upstreamNames) == 0 {
		return nil, nil
	}
	if len(upstreamNames) > 1 {
		return nil, fmt.Errorf("ambiguous tool choice across namespaces: %s", functionName)
	}
	function["name"], _ = json.Marshal(upstreamNames[0])
	functionRaw, err := json.Marshal(function)
	if err != nil {
		return nil, fmt.Errorf("convert responses tool_choice to chat tool_choice: %w", err)
	}
	obj["function"] = functionRaw
	result, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("convert responses tool_choice to chat tool_choice: %w", err)
	}
	return result, nil
}

func floatPtrValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func responsesInputToChatMessages(ctx context.Context, req *types.ResponsesRequest, toolAliases *responsesToolAliases) ([]map[string]any, error) {
	messages := []map[string]any{}
	if instructionText := types.ResponsesInstructionText(req.Instructions); instructionText != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructionText})
	}
	var asString string
	if err := json.Unmarshal(req.Input, &asString); err == nil {
		messages = append(messages, map[string]any{"role": "user", "content": asString})
		return messages, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(req.Input, &items); err != nil {
		return nil, fmt.Errorf("unsupported responses input shape")
	}
	// Keep pending reasoning until the next assistant message or tool-call turn.
	// Do not attach it to user/system messages.
	pendingReasoning := ""
	lastAssistantIdx := -1
	for _, item := range items {
		itemType, _ := item["type"].(string)
		switch itemType {
		case "message", "":
			role, _ := item["role"].(string)
			role = normalizeChatRole(role)
			content, err := normalizeResponsesContent(item["content"])
			if err != nil {
				return nil, err
			}
			message := map[string]any{"role": role, "content": content}
			if pendingReasoning != "" && role == "assistant" {
				message["reasoning_content"] = pendingReasoning
				pendingReasoning = ""
			}
			messages = append(messages, message)
			if role == "assistant" {
				lastAssistantIdx = len(messages) - 1
			}
		case "function_call":
			toolCall := map[string]any{
				"id":   item["call_id"],
				"type": "function",
				"function": map[string]any{
					"name":      responsesFunctionCallChatName(item, toolAliases),
					"arguments": item["arguments"],
				},
			}
			// Same-turn parallel function calls share one assistant message;
			// strict chat upstreams require each tool reply to follow the
			// single message that carries its tool_call.
			if open, ok := responsesOpenToolCallMessage(messages); ok {
				responsesAppendToolCall(open, toolCall)
				if pendingReasoning != "" {
					appendMessageReasoning(open, pendingReasoning)
					pendingReasoning = ""
				}
				break
			}
			message := map[string]any{
				"role":       "assistant",
				"content":    "",
				"tool_calls": []map[string]any{toolCall},
			}
			if pendingReasoning != "" {
				message["reasoning_content"] = pendingReasoning
				pendingReasoning = ""
			}
			messages = append(messages, message)
			lastAssistantIdx = len(messages) - 1
		case "function_call_output":
			content, err := normalizeResponsesContent(item["output"])
			if err != nil {
				return nil, err
			}
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": item["call_id"],
				"content":      content,
			})
		case "reasoning":
			reasoning := responsesReasoningItemText(item)
			if reasoning == "" {
				break
			}
			// Replayed history may record reasoning after the function_call
			// item it produced; attach it to that still-open tool-call
			// message instead of leaking it into the next turn.
			if open, ok := responsesOpenToolCallMessage(messages); ok {
				appendMessageReasoning(open, reasoning)
				break
			}
			if pendingReasoning == "" {
				pendingReasoning = reasoning
			} else {
				pendingReasoning += "\n" + reasoning
			}
		default:
			return nil, unsupportedResponsesFeature("input." + itemType)
		}
	}
	if pendingReasoning != "" {
		if lastAssistantIdx >= 0 {
			appendMessageReasoning(messages[lastAssistantIdx], pendingReasoning)
		} else {
			slog.DebugContext(ctx, "drop orphan reasoning input with no assistant target",
				slog.String("api", "/v1/responses"))
		}
	}
	return messages, nil
}

// responsesOpenToolCallMessage returns the most recent message when it is an
// assistant message holding only tool calls, i.e. a tool turn whose outputs
// have not been recorded yet. Same-turn parallel function_call items must
// share that message, and reasoning recorded right after a function_call
// belongs to it.
func responsesOpenToolCallMessage(messages []map[string]any) (map[string]any, bool) {
	if len(messages) == 0 {
		return nil, false
	}
	message := messages[len(messages)-1]
	if role, _ := message["role"].(string); role != "assistant" {
		return nil, false
	}
	if content, _ := message["content"].(string); content != "" {
		return nil, false
	}
	if _, ok := message["tool_calls"]; !ok {
		return nil, false
	}
	return message, true
}

func responsesAppendToolCall(message map[string]any, toolCall map[string]any) {
	calls, _ := message["tool_calls"].([]map[string]any)
	message["tool_calls"] = append(calls, toolCall)
}

func appendMessageReasoning(message map[string]any, reasoning string) {
	if existing, _ := message["reasoning_content"].(string); existing != "" {
		message["reasoning_content"] = existing + "\n" + reasoning
		return
	}
	message["reasoning_content"] = reasoning
}

// responsesFunctionCallChatName maps a historical function_call name to the chat
// tool name used upstream. When the call records its namespace, that identity is
// resolved exactly; otherwise the name is rewritten only when it uniquely maps to
// an aliased tool. Ambiguous or unknown names pass through unchanged.
func responsesFunctionCallChatName(item map[string]any, toolAliases *responsesToolAliases) string {
	name, _ := item["name"].(string)
	if name == "" || toolAliases == nil {
		return name
	}
	if namespace := responsesFunctionCallItemNamespace(item); namespace != "" {
		if upstreamName, ok := toolAliases.chatNameForIdentity(responsesToolIdentity{Name: name, Namespace: namespace}); ok {
			return upstreamName
		}
		return name
	}
	if upstreamName, aliased := toolAliases.chatName(name); aliased {
		return upstreamName
	}
	return name
}

func responsesFunctionCallItemNamespace(item map[string]any) string {
	if namespace, ok := item["namespace"].(string); ok && namespace != "" {
		return namespace
	}
	extra, _ := item["extra"].(map[string]any)
	if namespace, ok := extra["namespace"].(string); ok {
		return namespace
	}
	return ""
}

func responsesReasoningItemText(item map[string]any) string {
	if text := responsesContentText(item["summary"]); text != "" {
		return text
	}
	return responsesContentText(item["content"])
}

func responsesContentText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := responsesContentText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text, ok := v["text"]; ok {
			return responsesContentText(text)
		}
		if text, ok := v["content"]; ok {
			return responsesContentText(text)
		}
		return ""
	default:
		return ""
	}
}

func splitRawJSONArray(raw json.RawMessage) []json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var items []json.RawMessage
	_ = json.Unmarshal(raw, &items)
	return items
}

func normalizeResponsesContent(content any) (any, error) {
	parts, ok := content.([]any)
	if !ok {
		return content, nil
	}
	chatParts := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		obj, ok := part.(map[string]any)
		if !ok {
			continue
		}
		switch obj["type"] {
		case "input_text", "output_text", "text":
			chatParts = append(chatParts, map[string]any{"type": "text", "text": obj["text"]})
		case "input_image", "image_url":
			imageURL := normalizeResponsesImageURL(obj)
			chatParts = append(chatParts, map[string]any{"type": "image_url", "image_url": imageURL})
		case "input_audio":
			inputAudio := obj["input_audio"]
			if inputAudio == nil {
				inputAudio = map[string]any{
					"data":   obj["audio"],
					"format": obj["format"],
				}
			}
			chatParts = append(chatParts, map[string]any{"type": "input_audio", "input_audio": inputAudio})
		default:
			partType, _ := obj["type"].(string)
			if partType == "" {
				partType = "unknown"
			}
			return nil, unsupportedResponsesFeature("input.content." + partType)
		}
	}
	return chatParts, nil
}

func normalizeResponsesImageURL(obj map[string]any) any {
	imageURL := obj["image_url"]
	detail, hasDetail := obj["detail"]
	if s, ok := imageURL.(string); ok {
		normalized := map[string]any{"url": s}
		if hasDetail {
			normalized["detail"] = detail
		}
		return normalized
	}
	if imageURLObj, ok := imageURL.(map[string]any); ok && hasDetail {
		if _, exists := imageURLObj["detail"]; !exists {
			normalized := make(map[string]any, len(imageURLObj)+1)
			for key, value := range imageURLObj {
				normalized[key] = value
			}
			normalized["detail"] = detail
			return normalized
		}
	}
	return imageURL
}

func chatResponseToResponses(data []byte, publicModel string) (*types.ResponsesResponse, error) {
	return chatResponseToResponsesWithToolAliases(data, publicModel, nil)
}

func chatResponseToResponsesWithToolAliases(data []byte, publicModel string, toolAliases *responsesToolAliases) (*types.ResponsesResponse, error) {
	var chat types.ChatCompletion
	if err := json.Unmarshal(data, &chat); err != nil {
		return nil, err
	}
	reasoning := chatResponseReasoning(data)
	// TODO: If adapter mode later supports previous_response_id, pass the
	// public previous ID into this conversion and echo it in ResponsesResponse.
	resp := &types.ResponsesResponse{
		ID:        responsespkg.NewAdapterResponseID(),
		Object:    "response",
		CreatedAt: chat.Created,
		Status:    "completed",
		Model:     publicModel,
		Usage: &types.ResponsesUsage{
			InputTokens:  chat.Usage.PromptTokens,
			OutputTokens: chat.Usage.CompletionTokens,
			TotalTokens:  chat.Usage.TotalTokens,
		},
	}
	if resp.CreatedAt == 0 {
		resp.CreatedAt = time.Now().Unix()
	}
	if len(chat.Choices) == 0 {
		return resp, nil
	}
	msg := chat.Choices[0].Message
	if len(msg.ToolCalls) > 0 {
		for _, call := range msg.ToolCalls {
			identity := toolAliases.identity(call.Function.Name)
			item := types.ResponsesOutputItem{
				ID:        call.ID,
				Type:      "function_call",
				Status:    "completed",
				CallID:    call.ID,
				Name:      identity.Name,
				Arguments: call.Function.Arguments,
			}
			if identity.Namespace != "" {
				item.Extra = map[string]any{"namespace": identity.Namespace}
			}
			resp.Output = append(resp.Output, item)
		}
		appendResponsesReasoning(resp, reasoning)
		return resp, nil
	}
	if msg.Refusal != "" {
		resp.Output = append(resp.Output, types.ResponsesOutputItem{
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []types.ResponsesContentPart{{
				Type:    "refusal",
				Refusal: msg.Refusal,
			}},
		})
		appendResponsesReasoning(resp, reasoning)
		return resp, nil
	}
	text := msg.Content
	resp.OutputText = text
	resp.Output = append(resp.Output, types.ResponsesOutputItem{
		Type:   "message",
		Status: "completed",
		Role:   "assistant",
		Content: []types.ResponsesContentPart{{
			Type: "output_text",
			Text: text,
		}},
	})
	appendResponsesReasoning(resp, reasoning)
	return resp, nil
}

func chatResponseReasoning(data []byte) string {
	var raw struct {
		Choices []struct {
			Message map[string]json.RawMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw.Choices) == 0 {
		return ""
	}
	return reasoningFromRawFields(raw.Choices[0].Message)
}

func reasoningFromRawFields(fields map[string]json.RawMessage) string {
	if len(fields) == 0 {
		return ""
	}
	for _, key := range []string{"reasoning_content", "reasoning"} {
		var value string
		if err := json.Unmarshal(fields[key], &value); err == nil {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func appendResponsesReasoning(resp *types.ResponsesResponse, reasoning string) {
	if resp == nil || reasoning == "" {
		return
	}
	resp.Output = append(resp.Output, responsesReasoningOutputItem(reasoning))
}

func responsesReasoningOutputItem(reasoning string) types.ResponsesOutputItem {
	return types.ResponsesOutputItem{
		Type:   "reasoning",
		Status: "completed",
		Summary: []types.ResponsesSummaryPart{{
			Type: "summary_text",
			Text: reasoning,
		}},
	}
}
