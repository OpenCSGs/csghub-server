package decisions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestValidateSuccessResponse(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "valid response", body: `{"answers":[],"usage":{"input_tokens":1}}`},
		{name: "missing answers", body: `{"usage":{"input_tokens":1}}`, wantErr: true},
		{name: "missing usage", body: `{"answers":[]}`, wantErr: true},
		{name: "answers is null", body: `{"answers":null,"usage":{"input_tokens":1}}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response types.DecisionResponse
			require.NoError(t, json.Unmarshal([]byte(tt.body), &response))
			err := ValidateSuccessResponse(&response)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrMissingRequiredFields)
				return
			}
			require.NoError(t, err)
		})
	}
}
