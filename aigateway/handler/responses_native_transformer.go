package handler

import (
	"encoding/json"
	"fmt"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"strings"

	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

type responsesNativePayloadTransformer struct {
	mapper                   *responsespkg.IDMapper
	claims                   responsespkg.IDClaims
	publicPreviousResponseID string
	usage                    *types.ResponsesUsage
	idCache                  map[string]string
	responsesCounter         token.ResponsesTokenCounter
	logCapture               *responsespkg.LLMLogRecorder
}

func newResponsesNativePayloadTransformer(mapper *responsespkg.IDMapper, claims responsespkg.IDClaims, publicPreviousResponseID string, responsesCounter token.ResponsesTokenCounter, logCapture ...*responsespkg.LLMLogRecorder) *responsesNativePayloadTransformer {
	var recorder *responsespkg.LLMLogRecorder
	if len(logCapture) > 0 {
		recorder = logCapture[0]
	}
	return &responsesNativePayloadTransformer{
		mapper:                   mapper,
		claims:                   claims,
		publicPreviousResponseID: publicPreviousResponseID,
		responsesCounter:         responsesCounter,
		logCapture:               recorder,
	}
}

func (t *responsesNativePayloadTransformer) transformJSON(data []byte) ([]byte, bool, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, false, nil
	}
	changed, err := t.rewriteResponseIDs(obj)
	if err != nil {
		return nil, false, err
	}
	if !changed {
		t.captureResponsePayload(obj)
		return data, false, nil
	}
	t.captureResponsePayload(obj)
	rewritten, err := json.Marshal(obj)
	if err != nil {
		return nil, false, err
	}
	return rewritten, true, nil
}

func (t *responsesNativePayloadTransformer) rewriteResponseIDs(value any) (bool, error) {
	changed := false
	switch obj := value.(type) {
	case map[string]any:
		if t.publicPreviousResponseID != "" && obj["object"] == "response" {
			if _, exists := obj["previous_response_id"]; !exists {
				obj["previous_response_id"] = t.publicPreviousResponseID
				changed = true
			}
		}
		for key, raw := range obj {
			if (key == "id" || key == "response_id") && isRawUpstreamResponseID(raw) {
				id := raw.(string)
				wrapped, err := t.wrapUpstreamResponseID(id)
				if err != nil {
					return false, fmt.Errorf("wrap upstream response id: %w", err)
				}
				obj[key] = wrapped
				changed = true
				continue
			}
			childChanged, err := t.rewriteResponseIDs(raw)
			if err != nil {
				return false, err
			}
			changed = changed || childChanged
		}
	case []any:
		for _, item := range obj {
			childChanged, err := t.rewriteResponseIDs(item)
			if err != nil {
				return false, err
			}
			changed = changed || childChanged
		}
	}
	return changed, nil
}

func (t *responsesNativePayloadTransformer) wrapUpstreamResponseID(id string) (string, error) {
	if t.idCache == nil {
		t.idCache = map[string]string{}
	}
	if wrapped := t.idCache[id]; wrapped != "" {
		return wrapped, nil
	}
	claims := t.claims
	claims.UpstreamResponseID = id
	wrapped, err := t.mapper.Wrap(claims)
	if err != nil {
		return "", err
	}
	t.idCache[id] = wrapped
	return wrapped, nil
}

func isRawUpstreamResponseID(raw any) bool {
	id, ok := raw.(string)
	return ok && strings.HasPrefix(id, "resp_") && !responsespkg.IsGatewayResponseID(id) && !responsespkg.IsAdapterResponseID(id)
}

func (t *responsesNativePayloadTransformer) captureResponsePayload(obj map[string]any) {
	if obj == nil {
		return
	}
	// Native streams can include usage on multiple events; keep the latest block
	// because terminal response events normally carry the final aggregate usage.
	if usage, ok := parseResponsesUsage(obj["usage"]); ok {
		t.usage = usage
	}
	if response, ok := obj["response"].(map[string]any); ok {
		if usage, ok := parseResponsesUsage(response["usage"]); ok {
			t.usage = usage
		}
	}
	if t.logCapture != nil {
		t.logCapture.CapturePayloadMap(obj)
	}
	if t.responsesCounter == nil {
		return
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return
	}
	var event types.ResponsesStreamEvent
	if err := json.Unmarshal(data, &event); err == nil && event.Type != "" {
		t.responsesCounter.AppendEvent(event)
		return
	}
	var response types.ResponsesResponse
	if err := json.Unmarshal(data, &response); err == nil && response.Object == "response" {
		t.responsesCounter.Response(&response)
	}
}

func parseResponsesUsage(raw any) (*types.ResponsesUsage, bool) {
	if raw == nil {
		return nil, false
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var usage types.ResponsesUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return nil, false
	}
	if usage.InputTokensDetails != nil || usage.OutputTokensDetails != nil {
		return &usage, true
	}
	if usage.InputTokens != 0 || usage.OutputTokens != 0 || usage.TotalTokens != 0 {
		return &usage, true
	}
	return nil, false
}
