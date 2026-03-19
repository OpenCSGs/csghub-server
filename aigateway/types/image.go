package types

import (
	"encoding/json"

	"github.com/openai/openai-go/v3"
)

type ImageGenerationResponse struct {
	openai.ImagesResponse
}

type ImageGenerationRequest struct {
	openai.ImageGenerateParams
	RawJSON json.RawMessage `json:"-"`
}

// MarshalJSON merges ImageGenerateParams with RawJSON and marshals the result.
// RawJSON has json:"-" so it is omitted by default encoding; without this custom marshal,
// extra client fields (e.g. quality, style) stored in RawJSON would be dropped when sending to the backend.
func (r ImageGenerationRequest) MarshalJSON() ([]byte, error) {
	known, err := json.Marshal(r.ImageGenerateParams)
	if err != nil {
		return nil, err
	}
	if len(r.RawJSON) == 0 {
		return known, nil
	}
	var knownMap, rawMap map[string]any
	if err := json.Unmarshal(known, &knownMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(r.RawJSON, &rawMap); err != nil {
		return nil, err
	}
	for k, v := range rawMap {
		knownMap[k] = v
	}
	return json.Marshal(knownMap)
}
