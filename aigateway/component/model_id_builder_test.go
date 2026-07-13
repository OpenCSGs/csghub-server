package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestModelIDBuilder_To(t *testing.T) {
	builder := NewModelIDBuilder()

	tests := []struct {
		name   string
		deploy database.Deploy
		want   string
	}{
		{
			name: "serverless uses hf path first",
			deploy: database.Deploy{
				ID:   8765,
				Type: commontypes.ServerlessType,
				Repository: &database.Repository{
					HFPath: "hf-org/GLM-5.1-FP8",
					Path:   "zai-org/GLM-5.1-FP8",
					Name:   "GLM-5.1-FP8",
				},
			},
			want: "hf-org/GLM-5.1-FP8",
		},
		{
			name: "serverless falls back to repo path",
			deploy: database.Deploy{
				ID:   8765,
				Type: commontypes.ServerlessType,
				Repository: &database.Repository{
					Path: "zai-org/GLM-5.1-FP8",
					Name: "GLM-5.1-FP8",
				},
			},
			want: "zai-org/GLM-5.1-FP8",
		},
		{
			name: "inference uses repo name and base36 deploy id",
			deploy: database.Deploy{
				ID:   8765,
				Type: commontypes.InferenceType,
				Repository: &database.Repository{
					HFPath: "hf-user/Qwen3-0.6B",
					Path:   "JasonChiang1916/Qwen3-0.6B",
					Name:   "Qwen3-0.6B",
				},
			},
			want: "Qwen3-0.6B:6rh",
		},
		{
			name: "inference uses repo name as-is",
			deploy: database.Deploy{
				ID:   35,
				Type: commontypes.InferenceType,
				Repository: &database.Repository{
					Path: "namespace/model-from-path",
					Name: " model-from-name ",
				},
			},
			want: " model-from-name :z",
		},
		{
			name: "unknown type returns empty",
			deploy: database.Deploy{
				ID:   36,
				Type: 999,
				Repository: &database.Repository{
					Path: "namespace/model",
					Name: "model",
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := builder.To(tt.deploy)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModelIDBuilder_ToLegacyCSGHubModelID(t *testing.T) {
	builder := NewModelIDBuilder()

	assert.Equal(t, "hf/model:svc-name", builder.ToLegacyCSGHubModelID(&database.Repository{
		HFPath: "hf/model",
		Path:   "namespace/model",
	}, "svc-name"))
	assert.Equal(t, "namespace/model:svc-name", builder.ToLegacyCSGHubModelID(&database.Repository{
		Path: "namespace/model",
	}, "svc-name"))
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
