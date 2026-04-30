package types

import "encoding/json"

// ImageInputReferenceParam is the OpenAI-compatible image reference shape used
// by AIGateway's public image-to-video request API.
type ImageInputReferenceParam struct {
	FileID   string `json:"file_id,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

func (p ImageInputReferenceParam) IsZero() bool {
	return p.FileID == "" && p.ImageURL == ""
}

// VideoGenerationRequest is the OpenAI-compatible video generation request
// accepted by AIGateway. Provider adapters translate this public DTO into each
// provider's native text-to-video or image-to-video request shape.
type VideoGenerationRequest struct {
	Model          string                    `json:"model"`
	Prompt         string                    `json:"prompt"`
	Size           string                    `json:"size,omitempty"`
	Seconds        int64                     `json:"seconds,omitempty"`
	InputReference *ImageInputReferenceParam `json:"input_reference,omitempty"`

	// RawJSON preserves unknown OpenAI-compatible request fields so the default
	// OpenAI-compatible adapter can forward them instead of silently dropping
	// fields that AIGateway does not understand yet. Provider-native adapters may
	// intentionally ignore these fields when building provider-specific payloads.
	RawJSON json.RawMessage `json:"-"`
}

// UnmarshalJSON captures unknown request fields in RawJSON for passthrough to
// OpenAI-compatible backends.
func (r *VideoGenerationRequest) UnmarshalJSON(data []byte) error {
	type tempVideoGenerationRequest VideoGenerationRequest

	var temp tempVideoGenerationRequest
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}

	delete(allFields, "model")
	delete(allFields, "prompt")
	delete(allFields, "size")
	delete(allFields, "seconds")
	delete(allFields, "input_reference")

	*r = VideoGenerationRequest(temp)
	if len(allFields) > 0 {
		rawJSON, err := json.Marshal(allFields)
		if err != nil {
			return err
		}
		r.RawJSON = rawJSON
	}
	return nil
}

// MarshalJSON merges RawJSON back into the request when forwarding to an
// OpenAI-compatible backend.
func (r VideoGenerationRequest) MarshalJSON() ([]byte, error) {
	type tempVideoGenerationRequest VideoGenerationRequest
	data, err := json.Marshal(tempVideoGenerationRequest(r))
	if err != nil {
		return nil, err
	}

	if len(r.RawJSON) == 0 {
		return data, nil
	}

	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &knownFields); err != nil {
		return nil, err
	}

	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(r.RawJSON, &rawFields); err != nil {
		return nil, err
	}

	for k, v := range rawFields {
		knownFields[k] = v
	}

	return json.Marshal(knownFields)
}

// VideoError is the OpenAI-compatible error object embedded in video resources.
type VideoError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// VideoObject is the OpenAI-compatible video generation object returned by
// AIGateway after normalizing provider-specific responses.
type VideoObject struct {
	ID                 string      `json:"id"`
	Object             string      `json:"object,omitempty"`
	CreatedAt          int64       `json:"created_at,omitempty"`
	CompletedAt        *int64      `json:"completed_at,omitempty"`
	ExpiresAt          *int64      `json:"expires_at,omitempty"`
	Status             string      `json:"status,omitempty"`
	Model              string      `json:"model,omitempty"`
	Prompt             string      `json:"prompt,omitempty"`
	Size               string      `json:"size,omitempty"`
	Seconds            int64       `json:"seconds,omitempty"`
	Progress           *float64    `json:"progress,omitempty"`
	Error              *VideoError `json:"error,omitempty"`
	RemixedFromVideoID string      `json:"remixed_from_video_id,omitempty"`
}
