package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelConfigUnmarshalTopLevelFields(t *testing.T) {
	content := []byte(`{
		"architectures": ["Qwen2ForCausalLM"],
		"attention_dropout": 0.0,
		"bos_token_id": 151643,
		"eos_token_id": 151645,
		"hidden_act": "silu",
		"hidden_size": 896,
		"initializer_range": 0.02,
		"intermediate_size": 4864,
		"max_position_embeddings": 32768,
		"max_window_layers": 24,
		"model_type": "qwen2",
		"num_attention_heads": 14,
		"num_hidden_layers": 24,
		"num_key_value_heads": 2,
		"rms_norm_eps": 1e-06,
		"rope_theta": 1000000.0,
		"sliding_window": 32768,
		"tie_word_embeddings": true,
		"torch_dtype": "bfloat16",
		"transformers_version": "4.40.1",
		"use_cache": true,
		"use_sliding_window": false,
		"vocab_size": 151936
	}`)

	var config ModelConfig
	err := json.Unmarshal(content, &config)

	require.NoError(t, err)
	require.Equal(t, []string{"Qwen2ForCausalLM"}, config.Architectures)
	require.Equal(t, "qwen2", config.ModelType)
	require.Equal(t, 24, config.NumHiddenLayers)
	require.Equal(t, 896, config.HiddenSize)
	require.Equal(t, 14, config.NumAttentionHeads)
	require.Equal(t, "bfloat16", config.TorchDtype)
}

func TestModelConfigUnmarshalTextConfigFields(t *testing.T) {
	content := []byte(`{
		"architectures": ["Qwen3_5ForConditionalGeneration"],
		"model_type": "qwen3_5",
		"text_config": {
			"num_hidden_layers": 64,
			"hidden_size": 5120,
			"num_attention_heads": 24,
			"dtype": "bfloat16"
		},
		"vision_config": {
			"depth": 27,
			"hidden_size": 1152,
			"num_heads": 16
		}
	}`)

	var config ModelConfig
	err := json.Unmarshal(content, &config)

	require.NoError(t, err)
	require.Equal(t, []string{"Qwen3_5ForConditionalGeneration"}, config.Architectures)
	require.Equal(t, "qwen3_5", config.ModelType)
	require.Equal(t, 64, config.NumHiddenLayers)
	require.Equal(t, 5120, config.HiddenSize)
	require.Equal(t, 24, config.NumAttentionHeads)
	require.Equal(t, "bfloat16", config.TorchDtype)
}
