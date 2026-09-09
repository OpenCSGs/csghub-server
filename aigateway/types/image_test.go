package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageGenerationRequestPreservesUnknownFields(t *testing.T) {
	var request ImageGenerationRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"image-model","prompt":"hi","provider_option":true}`), &request))
	require.JSONEq(t, `{"provider_option":true}`, string(request.RawJSON))

	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"image-model","prompt":"hi","provider_option":true}`, string(encoded))
}
