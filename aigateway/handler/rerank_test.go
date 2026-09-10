package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestRerankRequest_JSONRoundTrip(t *testing.T) {
	raw := []byte(`{"model":"m1","query":"q","documents":["d1","d2"],"top_n":2,"return_documents":true,"custom_field":"custom_value"}`)

	var req types.RerankRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	assert.Equal(t, "m1", req.Model)
	assert.Equal(t, "q", req.Query)
	assert.Equal(t, []string{"d1", "d2"}, req.Documents)
	assert.Equal(t, int64(2), req.TopN)
	require.NotNil(t, req.ReturnDocuments)
	assert.True(t, *req.ReturnDocuments)

	// unknown fields must survive a marshal round trip
	data, err := json.Marshal(req)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "custom_value", out["custom_field"])
}
