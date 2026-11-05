package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoutingBodyRetentionPolicy(t *testing.T) {
	for _, model := range []string{"auto", "custom-auto", "concrete"} {
		raw := []byte(`{"model":"` + model + `","input":"hello","messages":[]}`)
		policy := func(name string) bool { return name == "custom-auto" }
		responses := ResponsesRequest{RetainRoutingBody: policy}
		messages := AnthropicMessagesRequest{RetainRoutingBody: policy}
		require.NoError(t, json.Unmarshal(raw, &responses))
		require.NoError(t, json.Unmarshal(raw, &messages))
		if model == "custom-auto" {
			require.Equal(t, raw, []byte(responses.RawBody))
			require.Equal(t, raw, []byte(messages.RawBody))
		} else {
			require.Empty(t, responses.RawBody)
			require.Empty(t, messages.RawBody)
		}
		out, err := json.Marshal(&responses)
		require.NoError(t, err)
		require.NotContains(t, string(out), "RetainRoutingBody")
	}
}
