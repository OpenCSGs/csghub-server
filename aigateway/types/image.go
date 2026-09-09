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

// UnmarshalJSON captures fields that are not defined by ImageGenerateParams.
func (r *ImageGenerationRequest) UnmarshalJSON(data []byte) error {
	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &r.ImageGenerateParams); err != nil {
		return err
	}

	known, err := json.Marshal(r.ImageGenerateParams)
	if err != nil {
		return err
	}
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(known, &knownFields); err != nil {
		return err
	}
	for key := range knownFields {
		delete(allFields, key)
	}
	if len(allFields) == 0 {
		r.RawJSON = nil
		return nil
	}
	r.RawJSON, err = json.Marshal(allFields)
	return err
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
