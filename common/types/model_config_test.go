package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelConfig_TotalExpertCount_NRoutedExperts(t *testing.T) {
	config := &ModelConfig{NRoutedExperts: 256}
	require.Equal(t, 256, config.TotalExpertCount())
}

func TestModelConfig_TotalExpertCount_NumExperts(t *testing.T) {
	config := &ModelConfig{NumExperts: 64}
	require.Equal(t, 64, config.TotalExpertCount())
}

func TestModelConfig_TotalExpertCount_NumLocalExperts(t *testing.T) {
	config := &ModelConfig{NumLocalExperts: 128}
	require.Equal(t, 128, config.TotalExpertCount())
}

func TestModelConfig_TotalExpertCount_Priority(t *testing.T) {
	// NRoutedExperts takes priority
	config := &ModelConfig{NRoutedExperts: 256, NumExperts: 64, NumLocalExperts: 128}
	require.Equal(t, 256, config.TotalExpertCount())

	// NumExperts takes second priority
	config = &ModelConfig{NumExperts: 64, NumLocalExperts: 128}
	require.Equal(t, 64, config.TotalExpertCount())
}

func TestModelConfig_TotalExpertCount_Dense(t *testing.T) {
	config := &ModelConfig{}
	require.Equal(t, 0, config.TotalExpertCount())
}

func TestModelConfig_TotalExpertCount_Nil(t *testing.T) {
	var config *ModelConfig
	require.Equal(t, 0, config.TotalExpertCount())
}

func TestModelConfig_ActiveExpertCount_NumExpertsPerTok(t *testing.T) {
	config := &ModelConfig{NumExpertsPerTok: 8}
	require.Equal(t, 8, config.ActiveExpertCount())
}

func TestModelConfig_ActiveExpertCount_NumActivatedExperts(t *testing.T) {
	config := &ModelConfig{NumActivatedExperts: 16}
	require.Equal(t, 16, config.ActiveExpertCount())
}

func TestModelConfig_ActiveExpertCount_Priority(t *testing.T) {
	config := &ModelConfig{NumExpertsPerTok: 8, NumActivatedExperts: 16}
	require.Equal(t, 8, config.ActiveExpertCount())
}

func TestModelConfig_ActiveExpertCount_None(t *testing.T) {
	config := &ModelConfig{}
	require.Equal(t, 0, config.ActiveExpertCount())
}

func TestModelConfig_ActiveExpertCount_Nil(t *testing.T) {
	var config *ModelConfig
	require.Equal(t, 0, config.ActiveExpertCount())
}

func TestModelConfig_IsMoE_True(t *testing.T) {
	config := &ModelConfig{NRoutedExperts: 256}
	require.True(t, config.IsMoE())
}

func TestModelConfig_IsMoE_False(t *testing.T) {
	config := &ModelConfig{}
	require.False(t, config.IsMoE())
}

func TestModelConfig_JSONParse_DeepSeekStyle(t *testing.T) {
	jsonStr := `{
		"architectures": ["DeepseekV3ForCausalLM"],
		"model_type": "deepseek_v3",
		"n_routed_experts": 256,
		"num_experts_per_tok": 8
	}`
	var config ModelConfig
	err := json.Unmarshal([]byte(jsonStr), &config)
	require.NoError(t, err)
	require.Equal(t, 256, config.TotalExpertCount())
	require.Equal(t, 8, config.ActiveExpertCount())
	require.True(t, config.IsMoE())
}

func TestModelConfig_JSONParse_MixtralStyle(t *testing.T) {
	jsonStr := `{
		"architectures": ["MixtralForCausalLM"],
		"model_type": "mixtral",
		"num_experts": 8,
		"num_experts_per_tok": 2
	}`
	var config ModelConfig
	err := json.Unmarshal([]byte(jsonStr), &config)
	require.NoError(t, err)
	require.Equal(t, 8, config.TotalExpertCount())
	require.Equal(t, 2, config.ActiveExpertCount())
	require.True(t, config.IsMoE())
}

func TestModelConfig_JSONParse_NemoStyle(t *testing.T) {
	jsonStr := `{
		"architectures": ["GPTModel"],
		"model_type": "gpt",
		"num_local_experts": 8,
		"num_activated_experts": 2
	}`
	var config ModelConfig
	err := json.Unmarshal([]byte(jsonStr), &config)
	require.NoError(t, err)
	require.Equal(t, 8, config.TotalExpertCount())
	require.Equal(t, 2, config.ActiveExpertCount())
	require.True(t, config.IsMoE())
}

func TestModelConfig_JSONParse_DenseModel(t *testing.T) {
	jsonStr := `{
		"architectures": ["LlamaForCausalLM"],
		"model_type": "llama",
		"num_hidden_layers": 32,
		"hidden_size": 4096
	}`
	var config ModelConfig
	err := json.Unmarshal([]byte(jsonStr), &config)
	require.NoError(t, err)
	require.Equal(t, 0, config.TotalExpertCount())
	require.False(t, config.IsMoE())
}
