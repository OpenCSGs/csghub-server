package types

// InferenceArch represents the allowed inference architectures configuration
type InferenceArch struct {
	ID        int    `json:"id"`
	Patterns  string `json:"patterns"` // Multiple regex patterns separated by newlines
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateInferenceArchReq represents the request to create/update inference arch configuration
type CreateInferenceArchReq struct {
	Patterns string `json:"patterns" binding:"required"`
}

// InferenceArchResponse represents the response for inference arch operations
type InferenceArchResponse struct {
	Code      int            `json:"code"`
	Msg       string         `json:"msg"`
	Data      *InferenceArch `json:"data"`
	Timestamp string         `json:"timestamp"`
	RequestId string         `json:"requestId"`
}
