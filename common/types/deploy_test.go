package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeployExtend_PDJSONBinding(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		expected DeployExtend
	}{
		{
			name:    "no PD field",
			jsonStr: `{"node_affinity":null,"tolerations":null}`,
			expected: DeployExtend{
				PD: nil,
			},
		},
		{
			name:    "PD disabled",
			jsonStr: `{"pd":{"enabled":false}}`,
			expected: DeployExtend{
				PD: &PDConfig{
					Enabled: false,
				},
			},
		},
		{
			name:    "PD enabled with replicas",
			jsonStr: `{"pd":{"enabled":true,"prefill_replicas":2,"decode_replicas":3}}`,
			expected: DeployExtend{
				PD: &PDConfig{
					Enabled:         true,
					PrefillReplicas: 2,
					DecodeReplicas:  3,
				},
			},
		},
		{
			name:    "PD enabled with HPA",
			jsonStr: `{"pd":{"enabled":true,"prefill_replicas":2,"decode_replicas":3,"hpa":{"enabled":true,"min_replicas":1,"max_replicas":4,"queue_threshold":5,"running_threshold":3,"scale_down_cooldown":300}}}`,
			expected: DeployExtend{
				PD: &PDConfig{
					Enabled:         true,
					PrefillReplicas: 2,
					DecodeReplicas:  3,
					HPA: &PDHPAConfig{
						Enabled:           true,
						MinReplicas:       1,
						MaxReplicas:       4,
						QueueThreshold:    5,
						RunningThreshold:  3,
						ScaleDownCooldown: 300,
					},
				},
			},
		},
		{
			name:    "PD enabled with nil HPA defaults to HPA enabled",
			jsonStr: `{"pd":{"enabled":true,"prefill_replicas":1,"decode_replicas":1}}`,
			expected: DeployExtend{
				PD: &PDConfig{
					Enabled:         true,
					PrefillReplicas: 1,
					DecodeReplicas:  1,
					HPA:             nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var de DeployExtend
			err := json.Unmarshal([]byte(tt.jsonStr), &de)
			require.NoError(t, err)

			if tt.expected.PD == nil {
				require.Nil(t, de.PD)
			} else {
				require.NotNil(t, de.PD)
				require.Equal(t, tt.expected.PD.Enabled, de.PD.Enabled)
				require.Equal(t, tt.expected.PD.PrefillReplicas, de.PD.PrefillReplicas)
				require.Equal(t, tt.expected.PD.DecodeReplicas, de.PD.DecodeReplicas)

				if tt.expected.PD.HPA == nil {
					require.Nil(t, de.PD.HPA)
				} else {
					require.NotNil(t, de.PD.HPA)
					require.Equal(t, tt.expected.PD.HPA.Enabled, de.PD.HPA.Enabled)
					require.Equal(t, tt.expected.PD.HPA.MinReplicas, de.PD.HPA.MinReplicas)
					require.Equal(t, tt.expected.PD.HPA.MaxReplicas, de.PD.HPA.MaxReplicas)
					require.Equal(t, tt.expected.PD.HPA.QueueThreshold, de.PD.HPA.QueueThreshold)
					require.Equal(t, tt.expected.PD.HPA.RunningThreshold, de.PD.HPA.RunningThreshold)
					require.Equal(t, tt.expected.PD.HPA.ScaleDownCooldown, de.PD.HPA.ScaleDownCooldown)
				}
			}
		})
	}
}

func TestPDConfig_ApplyDefaults(t *testing.T) {
	tests := []struct {
		name         string
		pd           PDConfig
		minReplica   int
		maxReplica   int
		expectedPD   PDConfig
		expectedHPA  PDHPAConfig
	}{
		{
			name: "defaults from SVCRequest min/max replica",
			pd: PDConfig{
				Enabled: true,
			},
			minReplica: 2,
			maxReplica: 5,
			expectedPD: PDConfig{
				Enabled:         true,
				PrefillReplicas: 2,
				DecodeReplicas:  2,
			},
			expectedHPA: PDHPAConfig{
				Enabled:           true,
				MinReplicas:       1,
				MaxReplicas:       5,
				QueueThreshold:    3,
				RunningThreshold:  100,
				ScaleDownCooldown: 300,
			},
		},
		{
			name: "defaults when min/max replica are 0",
			pd: PDConfig{
				Enabled: true,
			},
			minReplica: 0,
			maxReplica: 0,
			expectedPD: PDConfig{
				Enabled:         true,
				PrefillReplicas: 1,
				DecodeReplicas:  1,
			},
			expectedHPA: PDHPAConfig{
				Enabled:           true,
				MinReplicas:       1,
				MaxReplicas:       2,
				QueueThreshold:    3,
				RunningThreshold:  100,
				ScaleDownCooldown: 300,
			},
		},
		{
			name: "explicit values override defaults",
			pd: PDConfig{
				Enabled:         true,
				PrefillReplicas: 3,
				DecodeReplicas:  4,
				HPA: &PDHPAConfig{
					Enabled:           true,
					MinReplicas:       2,
					MaxReplicas:       10,
					QueueThreshold:    5,
					RunningThreshold:  4,
					ScaleDownCooldown: 600,
				},
			},
			minReplica: 2,
			maxReplica: 5,
			expectedPD: PDConfig{
				Enabled:         true,
				PrefillReplicas: 3,
				DecodeReplicas:  4,
			},
			expectedHPA: PDHPAConfig{
				Enabled:           true,
				MinReplicas:       2,
				MaxReplicas:       10,
				QueueThreshold:    5,
				RunningThreshold:  4,
				ScaleDownCooldown: 600,
			},
		},
		{
			name: "HPA nil defaults to enabled",
			pd: PDConfig{
				Enabled: true,
				HPA:     nil,
			},
			minReplica: 1,
			maxReplica: 3,
			expectedPD: PDConfig{
				Enabled:         true,
				PrefillReplicas: 1,
				DecodeReplicas:  1,
			},
			expectedHPA: PDHPAConfig{
				Enabled:           true,
				MinReplicas:       1,
				MaxReplicas:       3,
				QueueThreshold:    3,
				RunningThreshold:  100,
				ScaleDownCooldown: 300,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pd.ApplyDefaults(tt.minReplica, tt.maxReplica)
			require.Equal(t, tt.expectedPD.PrefillReplicas, tt.pd.PrefillReplicas)
			require.Equal(t, tt.expectedPD.DecodeReplicas, tt.pd.DecodeReplicas)
			require.NotNil(t, tt.pd.HPA)
			require.Equal(t, tt.expectedHPA.Enabled, tt.pd.HPA.Enabled)
			require.Equal(t, tt.expectedHPA.MinReplicas, tt.pd.HPA.MinReplicas)
			require.Equal(t, tt.expectedHPA.MaxReplicas, tt.pd.HPA.MaxReplicas)
			require.Equal(t, tt.expectedHPA.QueueThreshold, tt.pd.HPA.QueueThreshold)
			require.Equal(t, tt.expectedHPA.RunningThreshold, tt.pd.HPA.RunningThreshold)
			require.Equal(t, tt.expectedHPA.ScaleDownCooldown, tt.pd.HPA.ScaleDownCooldown)
		})
	}
}

func TestSVCRequest_PDThroughDeployExtend(t *testing.T) {
	jsonStr := `{
		"image_id": "vllm/vllm-openai:v0.6.0",
		"hardware": {"gpu": {"num": "2", "type": "A100"}, "memory": "32Gi", "replicas": 2},
		"env": {"port": "8000", "REPO_ID": "my-model"},
		"pd": {"enabled": true, "prefill_replicas": 2, "decode_replicas": 2}
	}`

	var req SVCRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	require.NoError(t, err)
	require.NotNil(t, req.PD)
	require.True(t, req.PD.Enabled)
	require.Equal(t, 2, req.PD.PrefillReplicas)
	require.Equal(t, 2, req.PD.DecodeReplicas)
}

// TestDeployRequest_PDNoShadow verifies that DeployRequest.PD (from embedded
// DeployExtend) is not shadowed by an explicit PD field. Before the fix,
// DeployRequest had both an explicit PD field and an embedded DeployExtend.PD,
// causing field shadowing: setting DeployExtend.PD was invisible when reading
// DeployRequest.PD. This test ensures they are the same field.
func TestDeployRequest_PDNoShadow(t *testing.T) {
	pdConfig := &PDConfig{
		Enabled:         true,
		PrefillReplicas: 2,
		DecodeReplicas:  3,
		Prefill: &PDRoleRuntimeConfig{
			TP: 2, EP: 1, DP: 1, TotalGPUs: 2,
		},
		Decode: &PDRoleRuntimeConfig{
			TP: 2, EP: 1, DP: 1, TotalGPUs: 2,
		},
	}

	dr := DeployRequest{}
	// Set PD via DeployExtend (the embedded field)
	dr.DeployExtend.PD = pdConfig

	// Reading dr.PD should return the same pointer (no shadowing)
	require.NotNil(t, dr.PD)
	require.Same(t, pdConfig, dr.PD)
	require.True(t, dr.PD.Enabled)
	require.Equal(t, 2, dr.PD.PrefillReplicas)
	require.Equal(t, 3, dr.PD.DecodeReplicas)

	// Setting dr.PD should also set DeployExtend.PD (same field)
	dr.PD.Enabled = false
	require.False(t, dr.DeployExtend.PD.Enabled)
}

// TestDeployRequest_PDJSONBinding verifies that JSON unmarshaling correctly
// populates DeployRequest.PD through the embedded DeployExtend field.
func TestDeployRequest_PDJSONBinding(t *testing.T) {
	jsonStr := `{
		"deploy_name": "test-deploy",
		"pd": {
			"enabled": true,
			"prefill_replicas": 2,
			"decode_replicas": 2,
			"prefill": {"tp": 2, "ep": 1, "dp": 1, "total_gpus": 2},
			"decode": {"tp": 2, "ep": 1, "dp": 1, "total_gpus": 2}
		}
	}`

	var dr DeployRequest
	err := json.Unmarshal([]byte(jsonStr), &dr)
	require.NoError(t, err)
	require.Equal(t, "test-deploy", dr.DeployName)

	// PD should be populated via embedded DeployExtend
	require.NotNil(t, dr.PD)
	require.True(t, dr.PD.Enabled)
	require.Equal(t, 2, dr.PD.PrefillReplicas)
	require.Equal(t, 2, dr.PD.DecodeReplicas)
	require.NotNil(t, dr.PD.Prefill)
	require.Equal(t, 2, dr.PD.Prefill.TP)
	require.NotNil(t, dr.PD.Decode)
	require.Equal(t, 2, dr.PD.Decode.TP)

	// Verify JSON marshaling round-trips correctly
	out, err := json.Marshal(dr)
	require.NoError(t, err)
	var dr2 DeployRequest
	err = json.Unmarshal(out, &dr2)
	require.NoError(t, err)
	require.NotNil(t, dr2.PD)
	require.True(t, dr2.PD.Enabled)
}
