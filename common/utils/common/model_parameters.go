package common

import (
	"encoding/json"
	"math"
	"slices"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

type safetensorInfo struct {
	shape    []int
	dtype    string
	elements int64
}

func parseSafetensorInfo(value any) (safetensorInfo, bool) {
	data, ok := value.(map[string]any)
	if !ok {
		return safetensorInfo{}, false
	}
	dtype, ok := data["dtype"].(string)
	if !ok || dtype == "" {
		return safetensorInfo{}, false
	}
	// Strictness is local to the optional correction; legacy parsing is unchanged.
	if dims, ok := data["shape"].([]any); ok {
		for _, raw := range dims {
			dim, ok := raw.(float64)
			if !ok || math.IsNaN(dim) || math.IsInf(dim, 0) || dim < 0 || dim >= float64(math.MaxInt) || math.Trunc(dim) != dim {
				return safetensorInfo{}, false
			}
		}
	}
	shape, err := extractShape(data)
	if err != nil {
		return safetensorInfo{}, false
	}
	var elements int64 = 1
	for _, dim := range shape {
		if dim < 0 || (dim != 0 && elements > math.MaxInt64/int64(dim)) {
			return safetensorInfo{}, false
		}
		elements *= int64(dim)
	}
	return safetensorInfo{shape: shape, dtype: dtype, elements: elements}, true
}

type mlxQuantization struct {
	Bits      int    `json:"bits"`
	GroupSize int    `json:"group_size"`
	Mode      string `json:"mode"`
	Method    string `json:"quant_method"`
}

// Unknown declarations simply leave the historical scan result unchanged.
func mlxParameterConfig(config *types.ModelConfig) (mlxQuantization, map[string]json.RawMessage, bool) {
	var quant mlxQuantization
	var layers map[string]json.RawMessage
	if config == nil {
		return quant, layers, false
	}
	raw := modelQuantizationDeclaration(config)
	if len(raw) == 0 || json.Unmarshal(raw, &quant) != nil || json.Unmarshal(raw, &layers) != nil || layers == nil {
		return quant, layers, false
	}
	return quant, layers, quant.Method == "" || quant.Method == "mlx"
}

// Apply an all-or-nothing correction to the legacy count. Storage and validity
// retain their original semantics, including when a correction cannot be proven.
func correctMLXParameters(total int64, tensors map[string]safetensorInfo, quant mlxQuantization, overrides map[string]json.RawMessage) (int64, bool) {
	replacements := make(map[string]int64)
	hasMXFP4 := false
	for name, weight := range tensors {
		if weight.dtype != "U32" || !strings.HasSuffix(name, ".weight") {
			continue
		}
		module := strings.TrimSuffix(name, ".weight")
		layerQuant := quant
		if raw, ok := overrides[module]; ok {
			layerQuant = mlxQuantization{}
			if json.Unmarshal(raw, &layerQuant) != nil {
				return 0, false
			}
		}
		hasMXFP4 = hasMXFP4 || layerQuant.Mode == "mxfp4"
		scales, haveScales := tensors[module+".scales"]
		biases, haveBiases := tensors[module+".biases"]
		if !haveScales || (layerQuant.Mode == "mxfp4" && haveBiases) || !validMLXWeight(weight, scales, biases, layerQuant) {
			return 0, false
		}
		replacements[name] = scales.elements * int64(layerQuant.GroupSize)
		replacements[module+".scales"] = 0
		if haveBiases {
			replacements[module+".biases"] = 0
		}
	}
	// An unpaired scale may belong to a packed tensor with an unsupported dtype.
	// Reject partial MXFP4 corrections without changing the affine-only path.
	for name := range tensors {
		if hasMXFP4 && (strings.HasSuffix(name, ".scales") || strings.HasSuffix(name, ".biases")) {
			if _, ok := replacements[name]; !ok {
				return 0, false
			}
		}
	}
	return applyParameterReplacements(total, tensors, replacements)
}

// MLX stores U32 packed weights: affine has a floating scale and bias per group;
// MXFP4 has a U8 E8M0 scale per 32 weights and no quantization bias.
// Check per-row packing consistency, including batched/MoE weights. This trusts
// the declared bits/group size: equal products (e.g. 64*4 and 32*8) cannot be
// distinguished from header shapes alone.
// https://ml-explore.github.io/mlx/build/html/python/_autosummary/mlx.core.quantize.html
func validMLXWeight(weight, scales, biases safetensorInfo, quant mlxQuantization) bool {
	if quant.Bits < 1 || quant.Bits > 8 || quant.Bits == 7 || (quant.GroupSize != 32 && quant.GroupSize != 64 && quant.GroupSize != 128) ||
		(quant.Method != "" && quant.Method != "mlx") {
		return false
	}
	if len(weight.shape) < 2 || len(scales.shape) != len(weight.shape) {
		return false
	}
	switch quant.Mode {
	case "", "affine":
		if !slices.Equal(scales.shape, biases.shape) || !isFloatScaleType(scales.dtype) || scales.dtype != biases.dtype {
			return false
		}
	case "mxfp4":
		if quant.Bits != 4 || quant.GroupSize != 32 || scales.dtype != "U8" {
			return false
		}
	default:
		return false
	}
	last := len(weight.shape) - 1
	if !slices.Equal(weight.shape[:last], scales.shape[:last]) || scales.elements == 0 ||
		scales.elements > math.MaxInt64/int64(quant.GroupSize) {
		return false
	}
	logicalWidth := int64(scales.shape[last]) * int64(quant.GroupSize)
	if logicalWidth > math.MaxInt64/int64(quant.Bits) || int64(weight.shape[last]) > math.MaxInt64/32 {
		return false
	}
	return logicalWidth*int64(quant.Bits) == int64(weight.shape[last])*32
}

func isFloatScaleType(dtype string) bool {
	return dtype == "F16" || dtype == "BF16" || dtype == "F32" || dtype == "F64"
}
