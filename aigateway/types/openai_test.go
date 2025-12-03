package types

import (
	"encoding/json"
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
