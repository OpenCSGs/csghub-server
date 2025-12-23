//go:build ee || saas

package csgbotcriticalerror

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
)

func TestGetLarkData_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	req := map[string]any{
		"receiver": map[string]any{
			"type": "chat_id",
			"id":   "oc_0bb5649827528a9bc45a8a3bc18a9387",
		},
		"payload": map[string]any{
			"service_name":  "api-service",
			"error_type":    "database_error",
			"error_level":   "critical",
			"location":      "/api/v1/users",
			"timestamp":     "2024-01-15T10:30:00Z",
			"environment":   "production",
			"error_message": "Database connection failed",
			"request_id":    "req-1234567890",
			"stack_trace":   "at com.example.Service.method(Service.java:123)",
		},
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	result, err := GetLarkData(ctx, cfg, msg)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Receiver)
	assert.False(t, result.Receiver.IsBroadcast)
	assert.Equal(t, "zh-CN", result.Receiver.GetLanguage())

	// Verify receiver IDs
	receiveIDs := result.Receiver.GetRecipients(notifychannel.RecipientKeyLarkReceiveIDs)
	assert.Len(t, receiveIDs, 1)
	assert.Equal(t, "chat_id:oc_0bb5649827528a9bc45a8a3bc18a9387", receiveIDs[0])

	// Verify payload
	assert.NotNil(t, result.Payload)
	payload := result.Payload.(map[string]any)
	assert.Equal(t, "api-service", payload["service_name"])
	assert.Equal(t, "database_error", payload["error_type"])
	assert.Equal(t, "critical", payload["error_level"])
	assert.Equal(t, "/api/v1/users", payload["location"])
	assert.Equal(t, "2024-01-15T10:30:00Z", payload["timestamp"])
	assert.Equal(t, "production", payload["environment"])
	assert.Equal(t, "Database connection failed", payload["error_message"])
	assert.Equal(t, "req-1234567890", payload["request_id"])
	assert.Equal(t, "at com.example.Service.method(Service.java:123)", payload["stack_trace"])

	// Verify Message is nil
	assert.Nil(t, result.Message)
}

func TestGetLarkData_WithOpenID(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	req := map[string]any{
		"receiver": map[string]any{
			"type": "open_id",
			"id":   "ou_1234567890abcdef",
		},
		"payload": map[string]any{
			"service_name": "api-service",
			"error_type":   "network_error",
		},
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	result, err := GetLarkData(ctx, cfg, msg)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Receiver)

	// Verify receiver IDs with open_id type
	receiveIDs := result.Receiver.GetRecipients(notifychannel.RecipientKeyLarkReceiveIDs)
	assert.Len(t, receiveIDs, 1)
	assert.Equal(t, "open_id:ou_1234567890abcdef", receiveIDs[0])
}

func TestGetLarkData_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	msg := types.ScenarioMessage{
		Parameters: "invalid json",
	}

	result, err := GetLarkData(ctx, cfg, msg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid parameters")
}

func TestGetLarkData_MissingPayload(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	req := map[string]any{
		"receiver": map[string]any{
			"type": "chat_id",
			"id":   "oc_0bb5649827528a9bc45a8a3bc18a9387",
		},
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	result, err := GetLarkData(ctx, cfg, msg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "payload field is required")
}

func TestGetLarkData_InvalidPayloadJSON(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	// Create JSON with invalid payload (not valid JSON)
	reqJSON := `{"receiver":{"type":"chat_id","id":"oc_123"},"payload":"invalid json string{not closed"}`
	msg := types.ScenarioMessage{
		Parameters: reqJSON,
	}

	result, err := GetLarkData(ctx, cfg, msg)

	assert.Error(t, err)
	assert.Nil(t, result)
	// Should fail either at initial unmarshal or at json.Valid check
	assert.True(t, err != nil)
}

func TestGetLarkData_PayloadNotValidJSON(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	// Create a request where payload is a string that's not valid JSON
	req := map[string]any{
		"receiver": map[string]any{
			"type": "chat_id",
			"id":   "oc_123",
		},
		"payload": "not a valid json object",
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	result, err := GetLarkData(ctx, cfg, msg)

	// The payload will be a JSON string "not a valid json object" which is valid JSON
	// but when we try to unmarshal it to map[string]any, it will fail
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to unmarshal payload")
}

func TestGetLarkData_MissingReceiver(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	req := map[string]any{
		"payload": map[string]any{
			"service_name": "api-service",
		},
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	result, err := GetLarkData(ctx, cfg, msg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "receiver field is required")
}

func TestGetLarkData_InvalidReceiverJSON(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	// Create JSON with invalid receiver structure
	reqJSON := `{"receiver":"invalid receiver","payload":{"service_name":"api-service"}}`
	msg := types.ScenarioMessage{
		Parameters: reqJSON,
	}

	result, err := GetLarkData(ctx, cfg, msg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid receiver")
}

func TestGetLarkData_EmptyPayload(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	req := map[string]any{
		"receiver": map[string]any{
			"type": "chat_id",
			"id":   "oc_0bb5649827528a9bc45a8a3bc18a9387",
		},
		"payload": map[string]any{},
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	result, err := GetLarkData(ctx, cfg, msg)

	// Empty payload map should be rejected
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "payload is empty")
}

func TestGetLarkData_NullPayload(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	// Create JSON with null payload
	// When unmarshaling to map[string]json.RawMessage, null becomes json.RawMessage("null")
	// which is valid JSON, but when unmarshaling to map[string]any, it becomes nil map
	// len(nil) returns 0 in Go, so it will be caught by the empty check
	reqJSON := `{"receiver":{"type":"chat_id","id":"oc_123"},"payload":null}`
	msg := types.ScenarioMessage{
		Parameters: reqJSON,
	}

	result, err := GetLarkData(ctx, cfg, msg)

	// When payload is null, unmarshaling to map[string]any results in nil
	// len(nil) returns 0, so it will be caught by the empty payload check
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "payload is empty")
}

func TestGetLarkData_NullReceiver(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	// Create JSON with null receiver
	// When unmarshaling null to a struct, it becomes a zero-value struct (not nil)
	// Receiver validation will be done in lark_ee.go, so GetLarkData should succeed
	reqJSON := `{"receiver":null,"payload":{"service_name":"api-service"}}`
	msg := types.ScenarioMessage{
		Parameters: reqJSON,
	}

	result, err := GetLarkData(ctx, cfg, msg)

	// Null receiver unmarshals to zero-value struct with empty type and id
	// GetLarkData doesn't validate receiver - validation happens in lark_ee.go
	assert.NoError(t, err)
	assert.NotNil(t, result)
	if result != nil {
		receiveIDs := result.Receiver.GetRecipients(notifychannel.RecipientKeyLarkReceiveIDs)
		// Should have one ID, formatted as ":" with empty type and id
		assert.Len(t, receiveIDs, 1)
		assert.Equal(t, ":", receiveIDs[0])
	}
}

func TestGetLarkData_EmptyReceiverType(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	req := map[string]any{
		"receiver": map[string]any{
			"type": "",
			"id":   "oc_123",
		},
		"payload": map[string]any{
			"service_name": "api-service",
		},
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	result, err := GetLarkData(ctx, cfg, msg)

	// Empty receiver type is not validated in GetLarkData - validation happens in lark_ee.go
	assert.NoError(t, err)
	assert.NotNil(t, result)
	if result != nil {
		receiveIDs := result.Receiver.GetRecipients(notifychannel.RecipientKeyLarkReceiveIDs)
		assert.Len(t, receiveIDs, 1)
		// Formatted ID will be ":oc_123" with empty type
		assert.Equal(t, ":oc_123", receiveIDs[0])
	}
}

func TestGetLarkData_EmptyReceiverID(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	req := map[string]any{
		"receiver": map[string]any{
			"type": "chat_id",
			"id":   "",
		},
		"payload": map[string]any{
			"service_name": "api-service",
		},
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	result, err := GetLarkData(ctx, cfg, msg)

	// Empty receiver ID is not validated in GetLarkData - validation happens in lark_ee.go
	assert.NoError(t, err)
	assert.NotNil(t, result)
	if result != nil {
		receiveIDs := result.Receiver.GetRecipients(notifychannel.RecipientKeyLarkReceiveIDs)
		assert.Len(t, receiveIDs, 1)
		// Formatted ID will be "chat_id:" with empty id
		assert.Equal(t, "chat_id:", receiveIDs[0])
	}
}
