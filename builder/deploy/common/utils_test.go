package common

import (
	"testing"

	"opencsg.com/csghub-server/common/types"

	"github.com/stretchr/testify/require"
)

func Test_GetNamespaceAndNameFromGitPath(t *testing.T) {
	testPath := "models_OpenCSG/wukong"
	ns, name, err := GetNamespaceAndNameFromGitPath(testPath)

	require.Nil(t, err)
	require.Equal(t, "OpenCSG", ns)
	require.Equal(t, "wukong", name)
}

func Test_GetValidSceneType(t *testing.T) {
	scenes := map[int]types.SceneType{
		types.SpaceType:      types.SceneSpace,
		types.InferenceType:  types.SceneModelInference,
		types.FinetuneType:   types.SceneModelFinetune,
		types.ServerlessType: types.SceneModelServerless,
		-1:                   types.SceneUnknow,
	}

	for scene, skuType := range scenes {
		res := GetValidSceneType(scene)
		require.Equal(t, skuType, res)
	}
}

func TestUpdateEvaluationEnvHardware(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		hardware types.HardWare
		expected map[string]string
	}{
		{
			name: "GPU set",
			env:  make(map[string]string),
			hardware: types.HardWare{
				Gpu: types.Processor{Num: "2"},
			},
			expected: map[string]string{
				"GPU_NUM": "2",
			},
		},
		{
			name: "NPU set when GPU is empty",
			env:  make(map[string]string),
			hardware: types.HardWare{
				Gpu: types.Processor{Num: ""},
				Npu: types.Processor{Num: "4"},
			},
			expected: map[string]string{
				"NPU_NUM": "4",
			},
		},
		{
			name: "Enflame set when GPU and NPU are empty",
			env:  make(map[string]string),
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: ""},
				Enflame: types.Processor{Num: "1"},
			},
			expected: map[string]string{
				"GCU_NUM": "1",
			},
		},
		{
			name: "Mlu set when others are empty",
			env:  make(map[string]string),
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: ""},
				Enflame: types.Processor{Num: ""},
				Mlu:     types.Processor{Num: "8"},
			},
			expected: map[string]string{
				"MLU_NUM": "8",
			},
		},
		{
			name: "DCU set when others are empty",
			env:  make(map[string]string),
			hardware: types.HardWare{
				Gpu: types.Processor{Num: ""},
				Npu: types.Processor{Num: ""},
				Gcu: types.Processor{Num: ""},
				Mlu: types.Processor{Num: ""},
				Dcu: types.Processor{Num: "1"},
			},
			expected: map[string]string{
				"DCU_NUM": "1",
			},
		},
		{
			name: "No hardware set",
			env:  make(map[string]string),
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: ""},
				Enflame: types.Processor{Num: ""},
				Mlu:     types.Processor{Num: ""},
			},
			expected: map[string]string{},
		},
		{
			name: "Existing env preserved",
			env: map[string]string{
				"EXISTING_KEY": "existing_value",
			},
			hardware: types.HardWare{
				Gpu: types.Processor{Num: "2"},
			},
			expected: map[string]string{
				"EXISTING_KEY": "existing_value",
				"GPU_NUM":      "2",
			},
		},
		{
			name: "First non-empty hardware wins (GPU)",
			env:  make(map[string]string),
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: "2"},
				Npu:     types.Processor{Num: "4"},
				Enflame: types.Processor{Num: "1"},
				Mlu:     types.Processor{Num: "8"},
			},
			expected: map[string]string{
				"GPU_NUM": "2",
			},
		},
		{
			name: "First non-empty hardware wins (NPU)",
			env:  make(map[string]string),
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: "4"},
				Enflame: types.Processor{Num: "1"},
				Mlu:     types.Processor{Num: "8"},
			},
			expected: map[string]string{
				"NPU_NUM": "4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateEvaluationEnvHardware(tt.env, tt.hardware)

			for k, v := range tt.expected {
				if tt.env[k] != v {
					t.Errorf("For key %s, expected %s, got %s", k, v, tt.env[k])
				}
			}
		})
	}
}

func TestGetXPUNumber(t *testing.T) {
	tests := []struct {
		name     string
		hardware types.HardWare
		want     int
		wantErr  bool
	}{
		{
			name:     "Empty hardware",
			hardware: types.HardWare{},
			want:     0,
			wantErr:  false,
		},
		{
			name: "Only GPU specified",
			hardware: types.HardWare{
				Gpu: types.Processor{Num: "4"},
			},
			want:    4,
			wantErr: false,
		},
		{
			name: "Only NPU specified",
			hardware: types.HardWare{
				Npu: types.Processor{Num: "2"},
			},
			want:    2,
			wantErr: false,
		},
		{
			name: "Only Gcu specified",
			hardware: types.HardWare{
				Enflame: types.Processor{Num: "8"},
			},
			want:    8,
			wantErr: false,
		},
		{
			name: "Only Mlu specified",
			hardware: types.HardWare{
				Mlu: types.Processor{Num: "1"},
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "GPU takes priority over others",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: "4"},
				Npu:     types.Processor{Num: "2"},
				Enflame: types.Processor{Num: "8"},
				Mlu:     types.Processor{Num: "1"},
			},
			want:    4,
			wantErr: false,
		},
		{
			name: "NPU takes priority when GPU is empty",
			hardware: types.HardWare{
				Npu:     types.Processor{Num: "2"},
				Enflame: types.Processor{Num: "8"},
				Mlu:     types.Processor{Num: "1"},
			},
			want:    2,
			wantErr: false,
		},
		{
			name: "Gcu takes priority when GPU and NPU are empty",
			hardware: types.HardWare{
				Enflame: types.Processor{Num: "8"},
				Mlu:     types.Processor{Num: "1"},
			},
			want:    8,
			wantErr: false,
		},
		{
			name: "Invalid number format",
			hardware: types.HardWare{
				Gpu: types.Processor{Num: "four"},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "Negative number",
			hardware: types.HardWare{
				Gpu: types.Processor{Num: "-1"},
			},
			want:    -1,
			wantErr: false,
		},
		{
			name: "Zero value",
			hardware: types.HardWare{
				Gpu: types.Processor{Num: "0"},
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "Large number",
			hardware: types.HardWare{
				Gpu: types.Processor{Num: "1024"},
			},
			want:    1024,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetXPUNumber(tt.hardware)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetXPUNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetXPUNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetXPUNumberEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		hardware types.HardWare
		want     int
		wantErr  bool
	}{
		{
			name: "Empty string in GPU",
			hardware: types.HardWare{
				Gpu: types.Processor{Num: ""},
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "Whitespace in number",
			hardware: types.HardWare{
				Gpu: types.Processor{Num: " 4 "},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "Multiple processors with empty strings",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: ""},
				Enflame: types.Processor{Num: ""},
				Mlu:     types.Processor{Num: ""},
			},
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetXPUNumber(tt.hardware)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetXPUNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetXPUNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceType(t *testing.T) {
	tests := []struct {
		name     string
		hardware types.HardWare
		expected types.ResourceType
	}{
		{
			name: "CPU when all hardware fields are empty",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: ""},
				Mlu:     types.Processor{Num: ""},
				Enflame: types.Processor{Num: ""},
			},
			expected: types.ResourceTypeCPU,
		},
		{
			name: "GPU when GPU Num is not empty",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: "1"},
				Npu:     types.Processor{Num: ""},
				Mlu:     types.Processor{Num: ""},
				Enflame: types.Processor{Num: ""},
			},
			expected: types.ResourceTypeGPU,
		},
		{
			name: "NPU when NPU Num is not empty and GPU is empty",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: "2"},
				Mlu:     types.Processor{Num: ""},
				Enflame: types.Processor{Num: ""},
			},
			expected: types.ResourceTypeNPU,
		},
		{
			name: "Mlu when Mlu Num is not empty and GPU/NPU are empty",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: ""},
				Mlu:     types.Processor{Num: "3"},
				Enflame: types.Processor{Num: ""},
			},
			expected: types.ResourceTypeMLU,
		},
		{
			name: "Gcu when Gcu Num is not empty and others are empty",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: ""},
				Mlu:     types.Processor{Num: ""},
				Enflame: types.Processor{Num: "4"},
			},
			expected: types.ResourceTypeGCU,
		},
		{
			name: "GPU has priority when multiple hardware types are present",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: "1"},
				Npu:     types.Processor{Num: "2"},
				Mlu:     types.Processor{Num: "3"},
				Enflame: types.Processor{Num: "4"},
			},
			expected: types.ResourceTypeGPU,
		},
		{
			name: "NPU has priority over Mlu and Gcu",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: "2"},
				Mlu:     types.Processor{Num: "3"},
				Enflame: types.Processor{Num: "4"},
			},
			expected: types.ResourceTypeNPU,
		},
		{
			name: "Mlu has priority over Gcu",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: ""},
				Npu:     types.Processor{Num: ""},
				Mlu:     types.Processor{Num: "3"},
				Enflame: types.Processor{Num: "4"},
			},
			expected: types.ResourceTypeMLU,
		},
		{
			name: "Zero values for Num fields",
			hardware: types.HardWare{
				Gpu:     types.Processor{Num: "0"},
				Npu:     types.Processor{Num: "0"},
				Mlu:     types.Processor{Num: "0"},
				Enflame: types.Processor{Num: "0"},
			},
			expected: types.ResourceTypeGPU,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ResourceType(tt.hardware)
			if actual != tt.expected {
				t.Errorf("ResourceType() = %v, want %v", actual, tt.expected)
			}
		})
	}
}
