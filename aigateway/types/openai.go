package types

import (
	"encoding/json"

	commontypes "opencsg.com/csghub-server/common/types"
)

// Provider type values for Metadata[MetaKeyLLMType].
const (
	ProviderTypeServerless  = "serverless"
	ProviderTypeInference   = "inference"
	ProviderTypeExternalLLM = "external_llm"
)

// Metadata key constants used when enriching model metadata.
const (
	MetaKeyLLMType           = "llm_type"
	MetaKeyPricing           = "pricing"
	MetaKeyPricingConfigured = "pricing_configured"
	MetaKeyRepoPath          = "repo_path"
	MetaKeyTasks             = "tasks"
)

// Resource ID format strings for external LLM (model ID) and CSGHub internal (path segment, repo path).
const (
	ExternalLLMResourceFmt = "thirdparty://%s"
	CSGHubResourceFmt      = "csghub://%s/%s"
)

// MeteringResource holds ResourceID, ResourceName, and CustomerID for metering events.
type MeteringResource struct {
	ResourceID   string
	ResourceName string
	CustomerID   string
}

// BaseModel represents the base model fields
type BaseModel struct {
	ID                  string         `json:"id"`
	Object              string         `json:"object"`
	Created             int64          `json:"created"` // organization-owner (e.g. openai)
	OwnedBy             string         `json:"owned_by"`
	Task                string         `json:"task"`                            // like text-generation, text-to-image etc
	SupportFunctionCall bool           `json:"support_function_call,omitempty"` // whether the model supports function calling
	IsPinned            *bool          `json:"is_pinned,omitempty"`             // whether the model is pinned
	Metadata            map[string]any `json:"metadata"`
}

// InternalModelInfo represents the internal model fields
type InternalModelInfo struct {
	CSGHubModelID    string               `json:"-"` // the internal model id (repo path) in CSGHub
	OwnerUUID        string               `json:"-"` // the uuid of deploy owner
	ClusterID        string               `json:"-"` // the deployed cluster id in CSGHub
	SvcName          string               `json:"-"` // the internal service name in CSGHub
	SvcType          int                  `json:"-"` // the internal service type like dedicated or serverless in CSGHub
	Hardware         commontypes.HardWare `json:"-"` // the deployed hardware
	RuntimeFramework string               `json:"-"` // the deployed framework
	ImageID          string               `json:"-"` // the deployed image id in CSGHub
}

// ExternalModelInfo represents the external model fields
type ExternalModelInfo struct {
	Provider string `json:"-"` // external provider name, like openai, anthropic etc
	AuthHead string `json:"-"` // the auth header to access the external model
	// NeedSensitiveCheck controls whether requests for this model should go
	// through sensitive content detection in aigateway. Set to false to skip
	// the check (e.g. for guard models or trusted internal models).
	NeedSensitiveCheck bool `json:"-"`
}

type Model struct {
	BaseModel
	InternalModelInfo                                   // internal model fields
	ExternalModelInfo                                   // external model fields
	Endpoint               string                       `json:"endpoint"`
	Upstreams              []commontypes.UpstreamConfig `json:"upstreams,omitempty"`
	RoutingPolicy          commontypes.RoutingPolicy    `json:"routing_policy"`
	Availability           *ModelAvailability           `json:"availability,omitempty"`
	UpstreamAvailabilities []UpstreamAvailability       `json:"upstream_availabilities,omitempty"`
	InternalUse            bool                         `json:"-"` // control whether the model is for internal use
}

type UpstreamAvailability struct {
	UpstreamID   int64          `json:"upstream_id,omitempty"`
	URL          string         `json:"url"`
	ModelName    string         `json:"model_name,omitempty"`
	Provider     string         `json:"provider,omitempty"`
	IsAvailable  bool           `json:"is_available"`
	HealthState  HealthState    `json:"health_state,omitempty"`
	CircuitState CircuitState   `json:"circuit_state,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	RawStatus    map[string]any `json:"-"`
}

func (m Model) MarshalJSON() ([]byte, error) {
	if m.InternalUse {
		// internalModelResponse
		type internalModelResponse struct {
			ID                  string                       `json:"id"`
			Object              string                       `json:"object"`
			Created             int64                        `json:"created"`
			OwnedBy             string                       `json:"owned_by"`
			Task                string                       `json:"task"`
			SupportFunctionCall *bool                        `json:"support_function_call,omitempty"`
			Endpoint            string                       `json:"endpoint"`
			Upstreams           []commontypes.UpstreamConfig `json:"upstreams,omitempty"`
			RoutingPolicy       commontypes.RoutingPolicy    `json:"routing_policy,omitempty"`
			Availability        *ModelAvailability           `json:"availability,omitempty"`
			UpstreamStatus      []UpstreamAvailability       `json:"upstream_availabilities,omitempty"`
			Metadata            map[string]any               `json:"metadata"`
			CSGHubModelID       *string                      `json:"csghub_model_id,omitempty"`
			OwnerUUID           *string                      `json:"owner_uuid,omitempty"`
			ClusterID           *string                      `json:"cluster_id,omitempty"`
			SvcName             *string                      `json:"svc_name,omitempty"`
			SvcType             *int                         `json:"svc_type,omitempty"`
			ImageID             *string                      `json:"image_id,omitempty"`
			RuntimeFramework    *string                      `json:"runtime_framework,omitempty"`
			AuthHead            *string                      `json:"auth_head,omitempty"`
			Provider            *string                      `json:"provider,omitempty"`
			NeedSensitiveCheck  bool                         `json:"need_sensitive_check"`
		}
		resp := internalModelResponse{
			ID:                 m.ID,
			Object:             m.Object,
			Created:            m.Created,
			OwnedBy:            m.OwnedBy,
			Task:               m.Task,
			Endpoint:           m.Endpoint,
			Upstreams:          m.Upstreams,
			RoutingPolicy:      m.RoutingPolicy,
			Availability:       m.Availability,
			UpstreamStatus:     m.UpstreamAvailabilities,
			Metadata:           m.Metadata,
			NeedSensitiveCheck: m.NeedSensitiveCheck,
		}

		if m.SupportFunctionCall {
			supportFC := m.SupportFunctionCall
			resp.SupportFunctionCall = &supportFC
		}
		if m.CSGHubModelID != "" {
			resp.CSGHubModelID = &m.CSGHubModelID
		}
		if m.OwnerUUID != "" {
			resp.OwnerUUID = &m.OwnerUUID
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
		if m.SvcType != 0 {
			resp.SvcType = &m.SvcType
		}
		if m.ImageID != "" {
			resp.ImageID = &m.ImageID
		}
		if m.RuntimeFramework != "" {
			resp.RuntimeFramework = &m.RuntimeFramework
		}

		return json.Marshal(resp)
	} else {
		type externalModelResponse struct {
			BaseModel
			Availability   *ModelAvailability     `json:"availability,omitempty"`
			UpstreamStatus []UpstreamAvailability `json:"upstream_availabilities,omitempty"`
		}
		return json.Marshal(externalModelResponse{
			BaseModel:      m.BaseModel,
			Availability:   m.Availability,
			UpstreamStatus: m.UpstreamAvailabilities,
		})
	}
}

func (m *Model) UnmarshalJSON(data []byte) error {
	type internalModelResponse struct {
		ID                  string                       `json:"id"`
		Object              string                       `json:"object"`
		Created             int64                        `json:"created"`
		OwnedBy             string                       `json:"owned_by"`
		Task                string                       `json:"task"`
		DisplayName         string                       `json:"display_name"`
		SupportFunctionCall bool                         `json:"support_function_call,omitempty"`
		Endpoint            string                       `json:"endpoint"`
		Upstreams           []commontypes.UpstreamConfig `json:"upstreams,omitempty"`
		RoutingPolicy       commontypes.RoutingPolicy    `json:"routing_policy,omitempty"`
		Availability        *ModelAvailability           `json:"availability,omitempty"`
		UpstreamStatus      []UpstreamAvailability       `json:"upstream_availabilities,omitempty"`
		Metadata            map[string]any               `json:"metadata"`
		CSGHubModelID       string                       `json:"csghub_model_id,omitempty"`
		OwnerUUID           string                       `json:"owner_uuid,omitempty"`
		ClusterID           string                       `json:"cluster_id,omitempty"`
		SvcName             string                       `json:"svc_name,omitempty"`
		SvcType             int                          `json:"svc_type,omitempty"`
		ImageID             string                       `json:"image_id,omitempty"`
		RuntimeFramework    string                       `json:"runtime_framework,omitempty"`
		AuthHead            string                       `json:"auth_head,omitempty"`
		Provider            string                       `json:"provider,omitempty"`
		NeedSensitiveCheck  bool                         `json:"need_sensitive_check"`
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
	m.Upstreams = aux.Upstreams
	m.RoutingPolicy = aux.RoutingPolicy
	m.Availability = aux.Availability
	m.UpstreamAvailabilities = aux.UpstreamStatus
	m.Metadata = aux.Metadata
	m.CSGHubModelID = aux.CSGHubModelID
	m.OwnerUUID = aux.OwnerUUID
	m.ClusterID = aux.ClusterID
	m.SvcName = aux.SvcName
	m.SvcType = aux.SvcType
	m.ImageID = aux.ImageID
	m.RuntimeFramework = aux.RuntimeFramework
	m.AuthHead = aux.AuthHead
	m.Provider = aux.Provider
	m.NeedSensitiveCheck = aux.NeedSensitiveCheck
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

// SkipBalance set the model for skip balance mode
func (m *Model) SkipBalance() bool {
	// MetaTaskKey values is array of strings, check if MetaTaskValGuard is in it
	if tasks, ok := m.Metadata[MetaTaskKey].([]interface{}); ok {
		for _, t := range tasks {
			if task, ok := t.(string); ok && task == MetaTaskValGuard {
				return true
			}
		}
	}
	return false
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

// ListModelsReq defines query-like parameters for listing models.
type ListModelsReq struct {
	ModelID            string   `json:"model_id"`
	Per                int      `json:"per"`
	Page               int      `json:"page"`
	LLMTypes           []string `json:"llm_types"` // filter by llm_type
	Task               string   `json:"task"`      // filter by task
	HasAssociatedModel *bool    `json:"has_associated_model"`
}

// UserPreferenceRequest defines the request parameters for UserPreference method
type UserPreferenceRequest struct {
	UserUUID string
	Scenario string
	Models   []Model
}

const OpenCSGAppNameHeader string = "OpenCSG-App-Name"

const (
	AgenticHubApp    = "Agentichub"
	MetaTaskKey      = "task"
	MetaTaskValGuard = "guard"
)

// ModelTokenPrice is currency plus per-million-token rate (major units, from accounting cents + sku_unit).
type ModelTokenPrice struct {
	Currency        string  `json:"currency"`
	PricePerMillion float64 `json:"price_per_million"`
}

// ModelModalPrice is a unit-based media generation price.
type ModelModalPrice struct {
	Currency      string  `json:"currency,omitempty"`
	PricePerUnit  float64 `json:"price_per_unit,omitempty"`
	SkuUnit       int64   `json:"sku_unit,omitempty"`
	SkuUnitType   string  `json:"sku_unit_type,omitempty"`
	Resolution    string  `json:"resolution,omitempty"`
	SkuResolution string  `json:"sku_resolution,omitempty"`
	SkuDesc       string  `json:"sku_desc,omitempty"`
}

// ModelScenePrice is Metadata["pricing"]: serverless and external_llm use token and modal generation prices.
type ModelScenePrice struct {
	InputTokenPrice  *ModelTokenPrice   `json:"input_token_price,omitempty"`
	OutputTokenPrice *ModelTokenPrice   `json:"output_token_price,omitempty"`
	TokenPrice       *ModelTokenPrice   `json:"token_price,omitempty"`
	ModalPrices      []*ModelModalPrice `json:"modal_prices,omitempty"`
}

// ModelAvailability represents the availability information for a model
type ModelAvailability struct {
	IsAvailable  bool         `json:"is_available"`
	HealthState  HealthState  `json:"health_state,omitempty"`
	CircuitState CircuitState `json:"circuit_state,omitempty"`
	Reason       string       `json:"reason,omitempty"`
}
