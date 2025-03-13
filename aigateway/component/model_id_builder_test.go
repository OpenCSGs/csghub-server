package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelIDBuilder_To(t *testing.T) {
	builder := ModelIDBuilder{}

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
	builder := ModelIDBuilder{}

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
			wantModelName: "",
			wantSvcName:   "",
			wantErr:       true,
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
