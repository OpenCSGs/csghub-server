package text2video

import (
	"encoding/json"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
)

const (
	videoAPITypeMiniMax  = "minimax"
	videoAPITypeSeedance = "seedance"
)

func videoAPIType(model *types.Model) string {
	if model == nil || model.Metadata == nil {
		return ""
	}
	raw, ok := model.Metadata["video_api"]
	if !ok {
		return ""
	}
	meta, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	apiType, _ := meta["type"].(string)
	return strings.ToLower(strings.TrimSpace(apiType))
}

func HasVideoAPIConfig(model *types.Model) bool {
	return videoAPIType(model) != ""
}

func isProviderType(model *types.Model, apiType string) bool {
	return videoAPIType(model) == apiType
}

func decodeJSON(body []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func stringAt(payload map[string]any, path string) string {
	if payload == nil || path == "" {
		return ""
	}
	var cur any = payload
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[part]
	}
	switch v := cur.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func mergeMetadata(base map[string]any, extra map[string]any) map[string]any {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	merged := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extra {
		if v != nil {
			merged[k] = v
		}
	}
	return merged
}

// MergeProviderMetadata merges provider-owned async generation metadata. Nil
// values are treated as absent; empty strings are preserved so callers decide
// whether an empty provider value is meaningful.
func MergeProviderMetadata(base map[string]any, extra map[string]any) map[string]any {
	return mergeMetadata(base, extra)
}

type RequestValidationError struct {
	Message string
}

func (e *RequestValidationError) Error() string {
	return e.Message
}
