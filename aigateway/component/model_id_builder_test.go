package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestModelIDBuilder_To(t *testing.T) {
	builder := NewModelIDBuilder()

	tests := []struct {
		name      string
		modelName string
		svcName   string
		want      string
	}{
		{
			name:      "normal case",
			modelName: "gpt-4",
			svcName:   "openai",
			want:      "gpt-4:openai",
		},
		{
			name:      "empty service name",
			modelName: "gpt-4",
			svcName:   "",
			want:      "gpt-4:",
		},
		{
			name:      "empty model name",
			modelName: "",
			svcName:   "openai",
			want:      ":openai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := builder.To(tt.modelName, tt.svcName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModelIDBuilder_From(t *testing.T) {
	builder := NewModelIDBuilder()

	tests := []struct {
		name          string
		modelID       string
		wantModelName string
		wantSvcName   string
		wantErr       bool
	}{
		{
			name:          "normal case",
			modelID:       "gpt-4:openai",
			wantModelName: "gpt-4",
			wantSvcName:   "openai",
			wantErr:       false,
		},
		{
			name:          "empty service name",
			modelID:       "gpt-4:",
			wantModelName: "gpt-4",
			wantSvcName:   "",
			wantErr:       false,
		},
		{
			name:          "empty model name",
			modelID:       ":openai",
			wantModelName: "",
			wantSvcName:   "openai",
			wantErr:       false,
		},
		{
			name:          "invalid format - no colon",
			modelID:       "invalid-format",
			wantModelName: "invalid-format",
			wantSvcName:   "",
			wantErr:       false,
		},
		{
			name:          "invalid format - multiple colons",
			modelID:       "gpt-4:openai:extra",
			wantModelName: "",
			wantSvcName:   "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotModelName, gotSvcName, err := builder.From(tt.modelID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantModelName, gotModelName)
			assert.Equal(t, tt.wantSvcName, gotSvcName)
		})
	}
}

func TestModelIDBuilder_GetModelOwner(t *testing.T) {
	builder := NewModelIDBuilder()

	tests := []struct {
		name       string
		deployType int
		username   string
		want       string
	}{
		{
			name:       "serverless model owner",
			deployType: commontypes.ServerlessType,
			username:   "user1",
			want:       "OpenCSG",
		},
		{
			name:       "inference model owner",
			deployType: commontypes.InferenceType,
			username:   "user2",
			want:       "user2",
		},
		{
			name:       "unknown type owner fallback to username",
			deployType: 999,
			username:   "user3",
			want:       "user3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := builder.GetModelOwner(tt.deployType, tt.username)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModelIDBuilder_BuildCompositeModelID(t *testing.T) {
	builder := NewModelIDBuilder()

	tests := []struct {
		name        string
		baseModelID string
		provider    string
		format      string
		want        string
	}{
		{
			name:        "default format",
			baseModelID: "gpt-4o",
			provider:    "openai",
			format:      "%s(%s)",
			want:        "gpt-4o(openai)",
		},
		{
			name:        "custom format",
			baseModelID: "claude-3",
			provider:    "anthropic",
			format:      "%s::%s",
			want:        "claude-3::anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := builder.BuildCompositeModelID(tt.baseModelID, tt.provider, tt.format)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModelIDBuilder_ParseCompositeModelID(t *testing.T) {
	builder := NewModelIDBuilder()

	tests := []struct {
		name    string
		modelID string
		format  string
		want    string
	}{
		{
			name:    "default format parse",
			modelID: "gpt-4o(openai)",
			format:  "%s(%s)",
			want:    "gpt-4o",
		},
		{
			name:    "base model id contains middle token",
			modelID: "a::b::provider",
			format:  "%s::%s",
			want:    "a::b",
		},
		{
			name:    "format not matched fallback default parser",
			modelID: "llama3(meta)",
			format:  "%s::%s",
			want:    "llama3",
		},
		{
			name:    "format and fallback both not matched",
			modelID: "plain-model-id",
			format:  "%s::%s",
			want:    "plain-model-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := builder.ParseCompositeModelID(tt.modelID, tt.format)
			assert.Equal(t, tt.want, got)
		})
	}
}
