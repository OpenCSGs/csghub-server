package types

import "opencsg.com/csghub-server/common/types"

// Model represents an AI model
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`

	// extend opanai struct
	Task          string `json:"task"` // like text-generation,text-to-image etc
	Endpoint      string `json:"-"`
	CSGHubModelID string `json:"-"` // the internal model id (repo path) in CSGHub
	SvcName       string `json:"-"` // the internal service name in CSGHub
	SvcType       int    `json:"-"` // the internal service type like dedicated or serverless in CSGHub

	Hardware         types.HardWare `json:"-"` // the deployed hardware
	RuntimeFramework string         `json:"-"` // the deployed framework
	ImageID          string         `json:"-"` // the deployed image
}

// ModelList represents the model list response
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
