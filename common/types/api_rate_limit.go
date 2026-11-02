package types

import "encoding/json"

type ApiRateLimit map[string]ApiRateLimitParam

type ApiRateLimitParam struct {
	Limit   int64 `json:"limit"`
	Window  int64 `json:"window"` // in seconds
	CheckIP bool  `json:"checkIP"`
}

// Implement encoding.BinaryMarshaler
// This is for WRITING to Redis
func (p *ApiRateLimitParam) MarshalBinary() ([]byte, error) {
	// Use json.Marshal to convert the struct to a JSON byte slice
	return json.Marshal(p)
}

// Implement encoding.BinaryUnmarshaler
// This is for READING from Redis
func (p *ApiRateLimitParam) UnmarshalBinary(data []byte) error {
	// Use json.Unmarshal to convert the JSON byte slice back to the struct
	return json.Unmarshal(data, p)
}

type ApiRateLimitCreateRequest struct {
	Path  string `json:"path" binding:"required"`
	Param struct {
		Limit   *int64 `json:"limit" binding:"required"`
		Window  *int64 `json:"window" ` // in seconds
		CheckIP *bool  `json:"checkIP" `
	} `json:"param" binding:"required"`
}
type ApiRateLimitUpdateRequest struct {
	Path  *string `json:"path"`
	Param struct {
		Limit   *int64 `json:"limit" `
		Window  *int64 `json:"window" ` // in seconds
		CheckIP *bool  `json:"checkIP" `
	} `json:"param"`
}
