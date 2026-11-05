package common

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

type parameterCorrection func(int64, map[string]safetensorInfo) (int64, bool)

type storedQuantization struct {
	Method      string `json:"quant_method"`
	Bits        int    `json:"bits"`
	GroupSize   int    `json:"group_size"`
	Version     string `json:"version"`
	ZeroPoint   bool   `json:"zero_point"`
	BlockSize   []int  `json:"weight_block_size"`
	ScaleFormat string `json:"scale_fmt"`
}

func modelQuantizationDeclaration(config *types.ModelConfig) json.RawMessage {
	if config == nil {
		return nil
	}
	for _, raw := range []json.RawMessage{config.Quantization, config.QuantizationConfig} {
		if len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return raw
		}
	}
	return nil
}

// Unsupported declarations leave the existing scanner untouched.
func modelParameterCorrection(config *types.ModelConfig) parameterCorrection {
	if quant, layers, ok := mlxParameterConfig(config); ok {
		return func(total int64, tensors map[string]safetensorInfo) (int64, bool) {
			return correctMLXParameters(total, tensors, quant, layers)
		}
	}
	var quant storedQuantization
	if json.Unmarshal(modelQuantizationDeclaration(config), &quant) != nil {
		return nil
	}
	switch quant.Method {
	case "mxfp4":
		return correctMXFP4Parameters
	case "awq":
		if quant.Bits != 4 || quant.Version != "gemm" || !quant.ZeroPoint || (quant.GroupSize <= 0 && quant.GroupSize != -1) {
			return nil
		}
		return func(total int64, tensors map[string]safetensorInfo) (int64, bool) {
			return correctAWQParameters(total, tensors, quant.GroupSize)
		}
	case "fp8":
		if len(quant.BlockSize) != 2 || quant.BlockSize[0] <= 0 || quant.BlockSize[1] <= 0 {
			return nil
		}
		deepseek := false
		for _, arch := range config.Architectures {
			deepseek = deepseek || arch == "DeepseekV4ForCausalLM" || arch == "DeepseekV41ForCausalLM"
		}
		deepseek = deepseek && quant.ScaleFormat == "ue8m0"
		return func(total int64, tensors map[string]safetensorInfo) (int64, bool) {
			return correctFP8Parameters(total, tensors, quant, deepseek)
		}
	}
	return nil
}

// AutoAWQ GEMM stores [in, out/8], zeros [in/group, out/8], scales [in/group, out].
// https://github.com/casper-hansen/AutoAWQ/blob/main/awq/modules/linear/gemm.py
func correctAWQParameters(total int64, tensors map[string]safetensorInfo, groupSize int) (int64, bool) {
	replacements := make(map[string]int64)
	for name, weight := range tensors {
		if !strings.HasSuffix(name, ".qweight") {
			continue
		}
		module := strings.TrimSuffix(name, ".qweight")
		scales, haveScales := tensors[module+".scales"]
		zeros, haveZeros := tensors[module+".qzeros"]
		if !haveScales || !haveZeros || weight.dtype != "I32" || zeros.dtype != "I32" || !isFloatScaleType(scales.dtype) ||
			len(weight.shape) != 2 || len(scales.shape) != 2 || len(zeros.shape) != 2 || weight.elements == 0 || weight.elements > math.MaxInt64/8 {
			return 0, false
		}
		group := groupSize
		if group == -1 {
			group = weight.shape[0]
		}
		if weight.shape[0]%group != 0 || zeros.shape[0] != weight.shape[0]/group || zeros.shape[1] != weight.shape[1] ||
			scales.shape[0] != zeros.shape[0] || int64(scales.shape[1]) != int64(weight.shape[1])*8 {
			return 0, false
		}
		replacements[name] = weight.elements * 8
		replacements[module+".scales"] = 0
		replacements[module+".qzeros"] = 0
	}
	for name := range tensors {
		if strings.HasSuffix(name, ".scales") || strings.HasSuffix(name, ".qzeros") {
			if _, ok := replacements[name]; !ok {
				return 0, false
			}
		}
	}
	return applyParameterReplacements(total, tensors, replacements)
}

// Only paired, shape-verified scales are removed. DeepSeek FP4 is recognized by
// architecture, ue8m0 scales and its [out,in/2] + [out,in/32] storage relation.
// https://huggingface.co/deepseek-ai/DeepSeek-V4.1-Flash/blob/main/inference/model.py
func correctFP8Parameters(total int64, tensors map[string]safetensorInfo, quant storedQuantization, deepseek bool) (int64, bool) {
	replacements := make(map[string]int64)
	for name, weight := range tensors {
		if !strings.HasSuffix(name, ".weight") {
			continue
		}
		fp8 := weight.dtype == "F8_E4M3" || weight.dtype == "F8_E5M2"
		if !fp8 && weight.dtype != "I8" {
			continue
		}
		if len(weight.shape) != 2 || weight.elements == 0 {
			return 0, false
		}
		module := strings.TrimSuffix(name, ".weight")
		scaleName := module + ".weight_scale_inv"
		if deepseek {
			scaleName = module + ".scale"
		}
		scale, ok := tensors[scaleName]
		if !ok || len(scale.shape) != 2 || scale.elements == 0 {
			return 0, false
		}
		if deepseek {
			if scale.dtype != "F8_E8M0" {
				return 0, false
			}
		} else if !isFloatScaleType(scale.dtype) {
			return 0, false
		}
		count := weight.elements
		if weight.dtype == "I8" {
			if !deepseek || weight.elements > math.MaxInt64/2 || weight.shape[1]%16 != 0 ||
				scale.shape[0] != weight.shape[0] || scale.shape[1] != weight.shape[1]/16 {
				return 0, false
			}
			count *= 2
		} else {
			rows := quant.BlockSize[0]
			// Engram embeddings use one scale per row and per block along K.
			if deepseek && strings.HasSuffix(module, ".engram.embed") {
				rows = 1
			}
			if scale.shape[0] != (weight.shape[0]-1)/rows+1 || scale.shape[1] != (weight.shape[1]-1)/quant.BlockSize[1]+1 {
				return 0, false
			}
		}
		replacements[name] = count
		replacements[scaleName] = 0
	}
	for name := range tensors {
		if strings.HasSuffix(name, ".weight_scale_inv") || (deepseek && strings.HasSuffix(name, ".scale")) {
			if _, ok := replacements[name]; !ok {
				return 0, false
			}
		}
	}
	return applyParameterReplacements(total, tensors, replacements)
}

// GPT-OSS packs 32 FP4 values into 16 U8 bytes with one U8 E8M0 scale.
// Transformers uses _blocks/_scales; OpenAI's original layout uses .blocks/.scales.
// https://github.com/openai/gpt-oss/blob/main/gpt_oss/torch/weights.py
func correctMXFP4Parameters(total int64, tensors map[string]safetensorInfo) (int64, bool) {
	replacements := make(map[string]int64)
	for name, blocks := range tensors {
		if !strings.HasSuffix(name, "_blocks") && !strings.HasSuffix(name, ".blocks") {
			continue
		}
		scaleName := strings.TrimSuffix(name, "blocks") + "scales"
		scales, ok := tensors[scaleName]
		if !ok || blocks.dtype != "U8" || scales.dtype != "U8" || len(blocks.shape) < 2 ||
			blocks.shape[len(blocks.shape)-1] != 16 || !slices.Equal(blocks.shape[:len(blocks.shape)-1], scales.shape) ||
			blocks.elements == 0 || blocks.elements > math.MaxInt64/2 {
			return 0, false
		}
		replacements[name] = blocks.elements * 2
		replacements[scaleName] = 0
	}
	for name := range tensors {
		if strings.HasSuffix(name, "_scales") || strings.HasSuffix(name, ".scales") {
			if _, ok := replacements[name]; !ok {
				return 0, false
			}
		}
	}
	return applyParameterReplacements(total, tensors, replacements)
}

// Validate the original sum and the complete replacement sum before committing.
// Returning false preserves both the original parameter count and ParamsValid.
func applyParameterReplacements(total int64, tensors map[string]safetensorInfo, replacements map[string]int64) (int64, bool) {
	if total < 0 || len(replacements) == 0 {
		return 0, false
	}
	var original, corrected int64
	for name, tensor := range tensors {
		if tensor.elements > math.MaxInt64-original {
			return 0, false
		}
		original += tensor.elements
		count, ok := replacements[name]
		if !ok {
			count = tensor.elements
		}
		if count < 0 || count > math.MaxInt64-corrected {
			return 0, false
		}
		corrected += count
	}
	return corrected, original == total
}
