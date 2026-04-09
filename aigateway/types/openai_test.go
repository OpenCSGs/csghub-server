package types

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

// TestModelSerialization tests the custom serialization of Model struct
func TestModelSerialization(t *testing.T) {
	model := &Model{
		BaseModel: BaseModel{
			ID:      "test-model",
			Object:  "model",
			Created: 1633046400,
			OwnedBy: "test-owner",
			Task:    "text-generation",

			SupportFunctionCall: true,
		},
		InternalModelInfo: InternalModelInfo{
			CSGHubModelID: "test/repo/path",
			OwnerUUID:     "test-owner-uuid",
			ClusterID:     "test-cluster-id",
			SvcName:       "test-service",
			SvcType:       1,
			ImageID:       "test-image-id",
		},
		ExternalModelInfo: ExternalModelInfo{
			AuthHead: "Bearer test-token",
			Provider: "test-provider",
		},
		Endpoint:    "http://test-endpoint.com",
		InternalUse: false,
	}

	// case1: response mode (InternalUse = false)
	t.Run("ExternalResponseMode", func(t *testing.T) {
		model.ForExternalResponse()
		jsonData, err := json.Marshal(model)
		if err != nil {
			t.Fatalf("Failed to marshal model in external response mode: %v", err)
		}

		jsonStr := string(jsonData)
		if contains(jsonStr, "endpoint") || contains(jsonStr, "internal_model_info") || contains(jsonStr, "external_model_info") {
			t.Errorf("External response should not contain sensitive fields, got: %s", jsonStr)
		}

		if !contains(jsonStr, "test-model") || !contains(jsonStr, "model") || !contains(jsonStr, "test-owner") {
			t.Errorf("External response should contain BaseModel fields, got: %s", jsonStr)
		}
	})

	// case2: internal use mode (InternalUse = true)
	t.Run("InternalUseMode", func(t *testing.T) {
		model.ForInternalUse()
		jsonData, err := json.Marshal(model)
		if err != nil {
			t.Fatalf("Failed to marshal model in internal use mode: %v", err)
		}
		jsonStr := string(jsonData)
		if !contains(jsonStr, "endpoint") || !contains(jsonStr, "http://test-endpoint.com") || !contains(jsonStr, "test-model") {
			t.Errorf("Internal response should contain base fields, got: %s", jsonStr)
		}

		if contains(jsonStr, "internal_model_info") {
			t.Errorf("InternalModelInfo should not be a nested object, got: %s", jsonStr)
		}
		if contains(jsonStr, "external_model_info") {
			t.Errorf("ExternalModelInfo should not be a nested object, got: %s", jsonStr)
		}
		if !contains(jsonStr, "auth_head") || !contains(jsonStr, "test-token") || !contains(jsonStr, "provider") || !contains(jsonStr, "test-provider") {
			t.Errorf("Internal response should contain expanded ExternalModelInfo fields, got: %s", jsonStr)
		}
		if !contains(jsonStr, "cluster_id") || !contains(jsonStr, "test-cluster-id") || !contains(jsonStr, "svc_name") || !contains(jsonStr, "test-service") {
			t.Errorf("Internal response should contain expanded InternalModelInfo fields, got: %s", jsonStr)
		}
		if !contains(jsonStr, "image_id") || !contains(jsonStr, "test-image-id") {
			t.Errorf("Internal response should contain expanded InternalModelInfo fields, got: %s", jsonStr)
		}
		// csghub_model_id, owner_uuid, and svc_type must survive the Redis round-trip so
		// that RecordUsage can populate resource_id for inference models.
		if !contains(jsonStr, "csghub_model_id") || !contains(jsonStr, "test/repo/path") {
			t.Errorf("Internal response should contain csghub_model_id, got: %s", jsonStr)
		}
		if !contains(jsonStr, "owner_uuid") {
			t.Errorf("Internal response should contain owner_uuid, got: %s", jsonStr)
		}
		if !contains(jsonStr, "svc_type") {
			t.Errorf("Internal response should contain svc_type, got: %s", jsonStr)
		}
	})

	// case3: mode switching
	t.Run("ModeSwitching", func(t *testing.T) {
		// switch to external response mode
		model.ForExternalResponse()
		externalJSON, _ := json.Marshal(model)

		// switch to internal use mode
		model.ForInternalUse()
		internalJSON, _ := json.Marshal(model)

		// verify that the serialized results are different
		if string(externalJSON) == string(internalJSON) {
			t.Errorf("Serialization should be different between internal and external modes")
		}
	})
}

// TestModelListSerialization
func TestModelListSerialization(t *testing.T) {
	// case1: serialize ModelList
	modelList := &ModelList{
		Object: "list",
		Data: []Model{
			{
				BaseModel: BaseModel{
					ID:     "model-1",
					Object: "model",
				},
				Endpoint:    "http://model-1.com",
				InternalUse: false,
			},
		},
	}

	// case2: serialize ModelList with nested models in external response mode
	jsonData, err := json.Marshal(modelList)
	if err != nil {
		t.Fatalf("Failed to marshal model list: %v", err)
	}

	// verify that the serialized results are correct
	jsonStr := string(jsonData)
	if !contains(jsonStr, "list") || !contains(jsonStr, "model-1") {
		t.Errorf("Model list should contain correct fields, got: %s", jsonStr)
	}

	// verify that the nested models follow the external response mode (sensitive fields are not included)
	if contains(jsonStr, "endpoint") || contains(jsonStr, "auth_head") {
		t.Errorf("Nested models in list should not contain sensitive fields by default, got: %s", jsonStr)
	}
}

// contains checks if the string s contains the substring substr
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestModelUnmarshal(t *testing.T) {
	// case1: unmarshal ModelList
	modelListJSON := `{
		"object": "list",
		"data": [
			{
				"id": "model-1",
				"object": "model",
				"created": 1633046400,
				"owned_by": "test-owner",
				"task": "text-generation",
				"support_function_call": true,
				"endpoint": "http://model-1.com",
				"internal_use": false
			}
		]
	}`

	var modelList ModelList
	err := json.Unmarshal([]byte(modelListJSON), &modelList)
	if err != nil {
		t.Fatalf("Failed to unmarshal model list: %v", err)
	}

	// verify that the unmarshaled results are correct
	if modelList.Object != "list" || len(modelList.Data) != 1 || modelList.Data[0].ID != "model-1" {
		t.Errorf("Model list unmarshal failed, got: %v", modelList)
	}
}

// TestInferenceModelRoundTrip verifies that CSGHubModelID, OwnerUUID, and SvcType
// survive a Redis marshal→unmarshal cycle so that RecordUsage can always populate
// resource_id for inference (llm_type=inference) models.
func TestInferenceModelRoundTrip(t *testing.T) {
	original := &Model{
		BaseModel: BaseModel{
			ID:      "Qwen/Qwen3Guard-Gen-0.6B:fgufi9nytc00",
			Object:  "model",
			Created: 1633046400,
			OwnedBy: "Qwen",
		},
		InternalModelInfo: InternalModelInfo{
			CSGHubModelID: "Qwen/Qwen3Guard-Gen-0.6B",
			OwnerUUID:     "uuid-owner-123",
			ClusterID:     "cluster-abc",
			SvcName:       "fgufi9nytc00",
			SvcType:       2,
			ImageID:       "img-xyz",
		},
		Endpoint:    "http://inference.internal/v1",
		InternalUse: true,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err, "marshal should not error")

	var restored Model
	require.NoError(t, json.Unmarshal(data, &restored), "unmarshal should not error")

	require.Equal(t, original.CSGHubModelID, restored.CSGHubModelID, "CSGHubModelID must round-trip")
	require.Equal(t, original.OwnerUUID, restored.OwnerUUID, "OwnerUUID must round-trip")
	require.Equal(t, original.SvcType, restored.SvcType, "SvcType must round-trip")
	require.Equal(t, original.ClusterID, restored.ClusterID, "ClusterID must round-trip")
	require.Equal(t, original.SvcName, restored.SvcName, "SvcName must round-trip")
	require.Equal(t, original.ImageID, restored.ImageID, "ImageID must round-trip")
	require.Equal(t, original.ID, restored.ID, "ID must round-trip")
	require.Equal(t, original.Endpoint, restored.Endpoint, "Endpoint must round-trip")
}

func TestModel_SkipBalance(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		expected bool
	}{
		{
			name:     "Metadata is nil",
			metadata: nil,
			expected: false,
		},
		{
			name:     "Metadata does not have MetaTaskKey",
			metadata: map[string]any{},
			expected: false,
		},
		{
			name:     "MetaTaskKey value is not a slice",
			metadata: map[string]any{MetaTaskKey: "not a slice"},
			expected: false,
		},
		{
			name:     "MetaTaskKey value is slice but not of strings",
			metadata: map[string]any{MetaTaskKey: []int{1, 2, 3}},
			expected: false,
		},
		{
			name:     "MetaTaskKey value is slice of strings but does not contain MetaTaskValGuard",
			metadata: map[string]any{MetaTaskKey: []interface{}{"text-generation", "text-to-image"}},
			expected: false,
		},
		{
			name:     "MetaTaskKey value is slice of strings and contains MetaTaskValGuard",
			metadata: map[string]any{MetaTaskKey: []interface{}{"text-generation", MetaTaskValGuard}},
			expected: true,
		},
		{
			name:     "MetaTaskKey value is slice of mixed types with MetaTaskValGuard",
			metadata: map[string]any{MetaTaskKey: []interface{}{1, "text-generation", MetaTaskValGuard, 3.14}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &Model{
				BaseModel: BaseModel{
					Metadata: tt.metadata,
				},
			}
			result := model.SkipBalance()
			require.Equal(t, tt.expected, result)
		})
	}
}
