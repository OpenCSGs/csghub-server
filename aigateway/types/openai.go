package types

// Model represents an AI model
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`

	// extend opanai struct
	Task     string `json:"task"` // like text-generation,text-to-image etc
	Endpoint string `json:"-"`
}

// ModelList represents the model list response
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}
