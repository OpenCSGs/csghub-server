package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// mtpIndex builds a minimal model.safetensors.index.json from tensor keys.
func mtpIndex(t *testing.T, keys ...string) []byte {
	t.Helper()
	weightMap := make(map[string]string, len(keys))
	for _, key := range keys {
		weightMap[key] = "model-00000-of-00009.safetensors"
	}
	data, err := json.Marshal(map[string]any{"metadata": map[string]int{"total_size": 1}, "weight_map": weightMap})
	require.NoError(t, err)
	return data
}

func deepseekConfig() *ModelConfig {
	return &ModelConfig{
		Architectures:         []string{"DeepseekV3ForCausalLM"},
		NumHiddenLayers:       61,
		NumNextNPredictLayers: 1,
	}
}

func TestHasMTPWeights(t *testing.T) {
	mtpLayerKeys := []string{
		"model.layers.60.self_attn.q_proj.weight",
		"model.layers.61.eh_proj.weight",
		"model.layers.61.enorm.weight",
		"model.layers.61.hnorm.weight",
		"model.layers.61.shared_head.head.weight",
		"model.layers.61.embed_tokens.weight",
	}

	t.Run("deepseek v3.1 style MTP block is detected", func(t *testing.T) {
		require.True(t, HasMTPWeights(deepseekConfig(), mtpIndex(t, mtpLayerKeys...)))
	})

	t.Run("missing num_hidden_layers fails closed", func(t *testing.T) {
		// Without a layer count the layer evidence cannot be anchored, so an
		// MTP-declaring config with a malformed config.json must never pass.
		cfg := deepseekConfig()
		cfg.NumHiddenLayers = 0
		require.False(t, HasMTPWeights(cfg, mtpIndex(t, mtpLayerKeys...)))
	})

	t.Run("quantization suffixes keep MTP keys detectable", func(t *testing.T) {
		keys := []string{
			"model.layers.60.mlp.experts.0.down_proj.weight_scale_inv",
			"model.layers.61.eh_proj.weight",
			"model.layers.61.enorm.weight",
			"model.layers.61.hnorm.weight",
			"model.layers.61.shared_head.head.weight",
		}
		require.True(t, HasMTPWeights(deepseekConfig(), mtpIndex(t, keys...)))
	})

	t.Run("glm style higher layer offsets are handled", func(t *testing.T) {
		cfg := &ModelConfig{NumHiddenLayers: 92, NumNextNPredictLayers: 1}
		keys := []string{
			"model.layers.91.self_attn.q_proj.weight",
			"model.layers.92.eh_proj.weight",
			"model.layers.92.enorm.weight",
			"model.layers.92.hnorm.weight",
			"model.layers.92.shared_head.head.weight",
		}
		require.True(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))
	})

	t.Run("multi layer MTP requires the full layer range", func(t *testing.T) {
		cfg := &ModelConfig{NumHiddenLayers: 61, NumNextNPredictLayers: 2}
		keys := []string{
			"model.layers.61.eh_proj.weight",
			"model.layers.61.enorm.weight",
			"model.layers.62.eh_proj.weight",
			"model.layers.62.enorm.weight",
		}
		require.True(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))

		// Only the first MTP layer present while config declares two.
		short := []string{
			"model.layers.60.self_attn.q_proj.weight",
			"model.layers.61.eh_proj.weight",
			"model.layers.61.enorm.weight",
		}
		require.False(t, HasMTPWeights(cfg, mtpIndex(t, short...)))
	})

	t.Run("multi layer MTP rejects layer without markers", func(t *testing.T) {
		// dlei 202358: layer 62 exists but carries only a regular tensor
		// (embed_tokens), not MTP markers. The second MTP layer is effectively
		// missing even though the tensor index reaches the expected max layer.
		cfg := &ModelConfig{NumHiddenLayers: 61, NumNextNPredictLayers: 2}
		keys := []string{
			"model.layers.61.eh_proj.weight",
			"model.layers.61.enorm.weight",
			"model.layers.62.embed_tokens.weight",
		}
		require.False(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))
	})

	t.Run("markers without extra layers are rejected", func(t *testing.T) {
		// Re-sharded checkpoint that keeps marker names below num_hidden_layers.
		keys := []string{
			"model.layers.60.eh_proj.weight",
			"model.layers.60.enorm.weight",
		}
		require.False(t, HasMTPWeights(deepseekConfig(), mtpIndex(t, keys...)))
	})

	t.Run("extra layers without markers are rejected", func(t *testing.T) {
		keys := []string{
			"model.layers.61.embed_tokens.weight",
			"model.layers.61.self_attn.q_proj.weight",
		}
		require.False(t, HasMTPWeights(deepseekConfig(), mtpIndex(t, keys...)))
	})

	t.Run("a single marker family is not enough", func(t *testing.T) {
		keys := []string{
			"model.layers.61.eh_proj.weight",
			"model.layers.61.embed_tokens.weight",
		}
		require.False(t, HasMTPWeights(deepseekConfig(), mtpIndex(t, keys...)))
	})

	t.Run("minimax style config without MTP weights is rejected", func(t *testing.T) {
		// MiniMax-M2: use_mtp=true in config.json but no MTP tensors shipped.
		cfg := &ModelConfig{NumHiddenLayers: 62, UseMTP: true, NumMTPModules: 3, MTPTransformerLayers: 1}
		keys := []string{
			"model.layers.61.self_attn.q_proj.weight",
			"model.layers.61.mlp.experts.0.down_proj.weight",
		}
		require.False(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))
	})

	t.Run("minimax style config with weights is accepted", func(t *testing.T) {
		// MiniMax-M2 declares num_mtp_modules=3, so layers 62/63/64 must all
		// carry MTP markers (vLLM uses num_mtp_modules as the layer count).
		cfg := &ModelConfig{NumHiddenLayers: 62, UseMTP: true, NumMTPModules: 3, MTPTransformerLayers: 1}
		keys := []string{
			"model.layers.61.self_attn.q_proj.weight",
			"model.layers.62.eh_proj.weight",
			"model.layers.62.enorm.weight",
			"model.layers.63.eh_proj.weight",
			"model.layers.63.enorm.weight",
			"model.layers.64.eh_proj.weight",
			"model.layers.64.enorm.weight",
		}
		require.True(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))
	})

	t.Run("no MTP declaration short-circuits to false", func(t *testing.T) {
		cfg := &ModelConfig{NumHiddenLayers: 61}
		require.False(t, HasMTPWeights(cfg, mtpIndex(t, mtpLayerKeys...)))
	})

	t.Run("nil config and malformed index fail closed", func(t *testing.T) {
		require.False(t, HasMTPWeights(nil, mtpIndex(t, mtpLayerKeys...)))
		require.False(t, HasMTPWeights(deepseekConfig(), []byte(`{"weight_map":`)))
		require.False(t, HasMTPWeights(deepseekConfig(), []byte(`{}`)))
	})

	t.Run("qwen3.5 style MTP block is detected", func(t *testing.T) {
		cfg := &ModelConfig{
			Architectures:      []string{"Qwen3ForCausalLM"},
			NumHiddenLayers:    28,
			MtpNumHiddenLayers: 1,
		}
		keys := []string{
			"model.layers.27.self_attn.q_proj.weight",
			"mtp.fc.weight",
			"mtp.norm.weight",
			"mtp.layers.0.self_attn.q_proj.weight",
			"mtp.layers.0.self_attn.k_proj.weight",
			"mtp.layers.0.mlp.down_proj.weight",
			"mtp.layers.0.input_layernorm.weight",
			"mtp.pre_fc_norm_embedding.weight",
			"mtp.pre_fc_norm_hidden.weight",
		}
		require.True(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))
	})

	t.Run("qwen3.5 style without weights is rejected", func(t *testing.T) {
		cfg := &ModelConfig{
			NumHiddenLayers:    28,
			MtpNumHiddenLayers: 1,
		}
		keys := []string{
			"model.layers.27.self_attn.q_proj.weight",
		}
		require.False(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))
	})

	t.Run("qwen3.5 style single marker is not enough", func(t *testing.T) {
		cfg := &ModelConfig{
			NumHiddenLayers:    28,
			MtpNumHiddenLayers: 1,
		}
		keys := []string{
			"mtp.layers.0.self_attn.q_proj.weight",
		}
		require.False(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))
	})

	t.Run("qwen3.5 style missing expected layer is rejected", func(t *testing.T) {
		cfg := &ModelConfig{
			NumHiddenLayers:    28,
			MtpNumHiddenLayers: 2,
		}
		keys := []string{
			"mtp.layers.0.self_attn.q_proj.weight",
			"mtp.layers.0.mlp.down_proj.weight",
			"mtp.fc.weight",
		}
		require.False(t, HasMTPWeights(cfg, mtpIndex(t, keys...)))
	})
}

func TestModelConfigMTPUnmarshal(t *testing.T) {
	var config ModelConfig
	require.NoError(t, json.Unmarshal([]byte(`{
		"architectures": ["DeepseekV3ForCausalLM"],
		"num_hidden_layers": 61,
		"num_nextn_predict_layers": 1
	}`), &config))
	require.Equal(t, 61, config.NumHiddenLayers)
	require.Equal(t, 1, config.NumNextNPredictLayers)
	require.True(t, config.MTPConfigured())

	var minimax ModelConfig
	require.NoError(t, json.Unmarshal([]byte(`{
		"model_type": "minimax_m2",
		"use_mtp": true,
		"num_mtp_modules": 3,
		"mtp_transformer_layers": 1
	}`), &minimax))
	require.True(t, minimax.UseMTP)
	require.Equal(t, 3, minimax.NumMTPModules)
	require.Equal(t, 1, minimax.MTPTransformerLayers)
	require.True(t, minimax.MTPConfigured())

	var plain ModelConfig
	require.NoError(t, json.Unmarshal([]byte(`{"architectures": ["LlamaForCausalLM"]}`), &plain))
	require.False(t, plain.MTPConfigured())

	var qwen35 ModelConfig
	require.NoError(t, json.Unmarshal([]byte(`{
		"architectures": ["Qwen3ForCausalLM"],
		"text_config": {
			"num_hidden_layers": 28,
			"mtp_num_hidden_layers": 1
		}
	}`), &qwen35))
	require.Equal(t, 28, qwen35.NumHiddenLayers)
	require.Equal(t, 1, qwen35.MtpNumHiddenLayers)
	require.True(t, qwen35.MTPConfigured())
}
