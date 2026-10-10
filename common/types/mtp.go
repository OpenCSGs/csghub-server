package types

import (
	"encoding/json"
	"regexp"
	"strconv"
)

// MTP module marker fragments only appear in the MTP block of a checkpoint
// (DeepSeek/GLM nextn layers, e.g. model.layers.61.eh_proj.weight). The layer
// regex anchors at the key start so expert indices (mlp.experts.<N>) cannot
// masquerade as layer numbers.
//
// Qwen3.5 uses a separate mtp.* namespace (mtp.layers.0.self_attn.* etc.)
// instead of the DeepSeek model.layers.N.* convention. Top-level keys
// (mtp.fc, mtp.norm, mtp.pre_fc_norm_*) are shared across all MTP layers.
var (
	mtpLayerKeyRe  = regexp.MustCompile(`^model\.layers\.(\d+)\.`)
	mtpMarkerKeyRe = regexp.MustCompile(`\.(eh_proj|enorm|hnorm|shared_head\.)`)

	qwenMtpLayerMarkerRe = regexp.MustCompile(`^mtp\.layers\.(\d+)\.([a-z_]+)\.`)
	qwenMtpTopMarkerRe   = regexp.MustCompile(`^mtp\.(fc|norm|pre_fc_norm)`)
)

// MTPConfigured reports whether config.json declares a multi-token prediction
// capability worth a weight-level check. It only reflects the declaration:
// some model families declare MTP fields without shipping the weights, so
// HasMTPWeights remains the decisive check.
func (c *ModelConfig) MTPConfigured() bool {
	return c != nil && (c.NumNextNPredictLayers > 0 || c.UseMTP || c.MtpNumHiddenLayers > 0)
}

// HasMTPWeights reports whether a model.safetensors.index.json weight_map
// carries multi-token prediction weights for the model described by cfg.
// Three independent pieces of evidence are required (fail-closed):
//
//  1. Layer range: every expected MTP layer (num_hidden_layers through
//     num_hidden_layers + expected - 1) must exist in the weight index.
//  2. Per-layer markers: each expected MTP layer must carry at least one
//     MTP module fragment, so a layer that exists only with regular tensors
//     (e.g. embed_tokens) does not satisfy the check (dlei 202358).
//  3. Marker diversity: at least two distinct MTP module fragments across
//     all MTP layers, so a config that merely declares MTP (e.g. MiniMax-M2)
//     without shipping the weights is rejected.
//
// Any parse failure or missing evidence returns false.
func HasMTPWeights(cfg *ModelConfig, indexJSON []byte) bool {
	if cfg == nil || !cfg.MTPConfigured() {
		return false
	}
	if cfg.NumHiddenLayers <= 0 {
		// Without the total layer count the "layer >= num_hidden_layers"
		// evidence degenerates: MTP markers could match tensors anywhere in
		// the main stack. Fail closed instead.
		return false
	}
	var index struct {
		WeightMap map[string]string `json:"weight_map"`
	}
	if err := json.Unmarshal(indexJSON, &index); err != nil || len(index.WeightMap) == 0 {
		return false
	}

	// Qwen3.5-style MTP: weights live under a separate mtp.* namespace
	// (mtp.layers.0.self_attn.*, mtp.fc.*, etc.) rather than model.layers.N.*.
	if cfg.MtpNumHiddenLayers > 0 {
		return hasQwenMTPWeights(cfg, index.WeightMap)
	}

	expectedLayers := cfg.NumNextNPredictLayers
	if expectedLayers <= 0 {
		// MiniMax-style fallback (use_mtp + num_mtp_modules). vLLM uses
		// num_mtp_modules as the MTP layer count (layers base..base+N-1);
		// mtp_transformer_layers is the per-module depth, not the count.
		expectedLayers = cfg.NumMTPModules
		if expectedLayers <= 0 {
			expectedLayers = 1
		}
	}

	// Collect MTP markers per layer so every expected layer can be verified
	// individually.
	layerMarkers := make(map[int]map[string]bool)
	for key := range index.WeightMap {
		m := mtpLayerKeyRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		layer, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if layer < cfg.NumHiddenLayers {
			continue
		}
		if marker := mtpMarkerKeyRe.FindStringSubmatch(key); marker != nil {
			if layerMarkers[layer] == nil {
				layerMarkers[layer] = make(map[string]bool)
			}
			layerMarkers[layer][marker[1]] = true
		}
	}

	// Each expected MTP layer must have at least one marker.
	allMarkers := make(map[string]bool)
	for i := 0; i < expectedLayers; i++ {
		layer := cfg.NumHiddenLayers + i
		markers, ok := layerMarkers[layer]
		if !ok || len(markers) == 0 {
			return false
		}
		for m := range markers {
			allMarkers[m] = true
		}
	}
	return len(allMarkers) >= 2
}

// hasQwenMTPWeights checks for Qwen3.5-style MTP weights under the mtp.*
// namespace. The same fail-closed principle applies: every expected layer
// (0-indexed) must carry at least one marker, and at least two distinct
// markers must exist across all layers plus top-level keys.
func hasQwenMTPWeights(cfg *ModelConfig, weightMap map[string]string) bool {
	expectedLayers := cfg.MtpNumHiddenLayers
	if expectedLayers <= 0 {
		expectedLayers = 1
	}

	layerMarkers := make(map[int]map[string]bool)
	topMarkers := make(map[string]bool)
	for key := range weightMap {
		if m := qwenMtpLayerMarkerRe.FindStringSubmatch(key); m != nil {
			layer, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			if layerMarkers[layer] == nil {
				layerMarkers[layer] = make(map[string]bool)
			}
			layerMarkers[layer][m[2]] = true
			continue
		}
		if m := qwenMtpTopMarkerRe.FindStringSubmatch(key); m != nil {
			topMarkers[m[1]] = true
		}
	}

	allMarkers := make(map[string]bool)
	for m := range topMarkers {
		allMarkers[m] = true
	}
	for i := 0; i < expectedLayers; i++ {
		markers, ok := layerMarkers[i]
		if !ok || len(markers) == 0 {
			return false
		}
		for m := range markers {
			allMarkers[m] = true
		}
	}
	return len(allMarkers) >= 2
}
