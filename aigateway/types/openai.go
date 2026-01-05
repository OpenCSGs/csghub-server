package types

import (
	"encoding/json"

	"opencsg.com/csghub-server/common/types"
)

// BaseModel represents the base model fields
type BaseModel struct {
	ID                  string `json:"id"`
	Object              string `json:"object"`
	Created             int64  `json:"created"` // organization-owner (e.g. openai)
	OwnedBy             string `json:"owned_by"`
	Task                string `json:"task"`                            // like text-generation, text-to-image etc
	SupportFunctionCall bool   `json:"support_function_call,omitempty"` // whether the model supports function calling
}

// InternalModelInfo represents the internal model fields
type InternalModelInfo struct {
	CSGHubModelID    string         `json:"-"` // the internal model id (repo path) in CSGHub
	OwnerUUID        string         `json:"-"` // the uuid of deploy owner
	ClusterID        string         `json:"-"` // the deployed cluster id in CSGHub
	SvcName          string         `json:"-"` // the internal service name in CSGHub
	SvcType          int            `json:"-"` // the internal service type like dedicated or serverless in CSGHub
	Hardware         types.HardWare `json:"-"` // the deployed hardware
	RuntimeFramework string         `json:"-"` // the deployed framework
	ImageID          string         `json:"-"` // the deployed image id in CSGHub
}

// ExternalModelInfo represents the external model fields
type ExternalModelInfo struct {
	Provider string `json:"-"` // external provider name, like openai, anthropic etc
	AuthHead string `json:"-"` // the auth header to access the external model
}

type Model struct {
	BaseModel
	InternalModelInfo        // internal model fields
	ExternalModelInfo        // external model fields
	Endpoint          string `json:"endpoint"`
	InternalUse       bool   `json:"-"` // control whether the model is for internal use
}

func (m Model) MarshalJSON() ([]byte, error) {
	if m.InternalUse {
		// internalModelResponse
		type internalModelResponse struct {
			ID                  string  `json:"id"`
			Object              string  `json:"object"`
			Created             int64   `json:"created"`
			OwnedBy             string  `json:"owned_by"`
			Task                string  `json:"task"`
			SupportFunctionCall *bool   `json:"support_function_call,omitempty"`
			Endpoint            string  `json:"endpoint"`
			ClusterID           *string `json:"cluster_id,omitempty"`
			SvcName             *string `json:"svc_name,omitempty"`
			ImageID             *string `json:"image_id,omitempty"`
			AuthHead            *string `json:"auth_head,omitempty"`
			Provider            *string `json:"provider,omitempty"`
		}
		resp := internalModelResponse{
			ID:       m.ID,
			Object:   m.Object,
			Created:  m.Created,
			OwnedBy:  m.OwnedBy,
			Task:     m.Task,
			Endpoint: m.Endpoint,
		}

		if m.SupportFunctionCall {
			supportFC := m.SupportFunctionCall
			resp.SupportFunctionCall = &supportFC
		}
		if m.Provider != "" {
			resp.Provider = &m.Provider
		}
		if m.AuthHead != "" {
			resp.AuthHead = &m.AuthHead
		}
		if m.ClusterID != "" {
			resp.ClusterID = &m.ClusterID
		}
		if m.SvcName != "" {
			resp.SvcName = &m.SvcName
		}
		if m.ImageID != "" {
			resp.ImageID = &m.ImageID
		}

		return json.Marshal(resp)
	} else {
		return json.Marshal(m.BaseModel)
	}
}

func (m *Model) UnmarshalJSON(data []byte) error {
	type internalModelResponse struct {
		ID                  string `json:"id"`
		Object              string `json:"object"`
		Created             int64  `json:"created"`
		OwnedBy             string `json:"owned_by"`
		Task                string `json:"task"`
		SupportFunctionCall bool   `json:"support_function_call,omitempty"`
		Endpoint            string `json:"endpoint"`
		ClusterID           string `json:"cluster_id,omitempty"`
		SvcName             string `json:"svc_name,omitempty"`
		ImageID             string `json:"image_id,omitempty"`
		AuthHead            string `json:"auth_head,omitempty"`
		Provider            string `json:"provider,omitempty"`
	}
	var aux internalModelResponse
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.ID = aux.ID
	m.Object = aux.Object
	m.Created = aux.Created
	m.OwnedBy = aux.OwnedBy
	m.Task = aux.Task
	m.SupportFunctionCall = aux.SupportFunctionCall
	m.Endpoint = aux.Endpoint
	m.ClusterID = aux.ClusterID
	m.SvcName = aux.SvcName
	m.ImageID = aux.ImageID
	m.AuthHead = aux.AuthHead
	m.Provider = aux.Provider
	return nil
}

// ForInternalUse set the model for internal use mode
func (m *Model) ForInternalUse() *Model {
	m.InternalUse = true
	return m
}

// ForExternalResponse set the model for external response mode
func (m *Model) ForExternalResponse() *Model {
	m.InternalUse = false
	return m
}

// ModelList represents the model list response
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
	// Pagination metadata
	FirstID    *string `json:"first_id,omitempty"`
	LastID     *string `json:"last_id,omitempty"`
	HasMore    bool    `json:"has_more"`
	TotalCount int     `json:"total_count"`
}
