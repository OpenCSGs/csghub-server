package common

import (
	"encoding/json"
	"opencsg.com/csghub-server/common/types"
	"testing"
)

func TestGetModelInfo_LegacyFallback(t *testing.T) {
	for _, raw := range []string{``, `{"quant_method":"gptq"}`, `{"quant_method":"awq"}`, `{"quant_method":"unknown"}`, `{"bits":6}`, `{"bits":4,"group_size":128}`} {
		t.Run(raw, func(t *testing.T) {
			restore := patchFetchSafetensorsMetadata(func(string) (map[string]any, error) {
				return map[string]any{"layer.weight": map[string]any{"dtype": "U32", "shape": []any{2.0, 16.0}}, "layer.scales": map[string]any{"dtype": "BF16", "shape": []any{2.0, 1.0}}}, nil
			})
			defer restore()
			got, err := GetModelInfo([]string{"one", "two"}, 512, &types.ModelConfig{QuantizationConfig: json.RawMessage(raw)})
			if err != nil {
				t.Fatal(err)
			}
			if !got.ParamsValid || got.TotalParams != 68 {
				t.Fatalf("legacy count changed: %+v", got)
			}
		})
	}
}

func TestGetModelInfo_UnchangedNonMLX(t *testing.T) {
	for _, dtype := range []string{"BF16", "F8_E4M3", "I8"} {
		for _, raw := range []string{``, `{"quant_method":"compressed-tensors","format":"dense"}`, `{"quant_method":"bitsandbytes","load_in_8bit":true}`} {
			t.Run(dtype+raw, func(t *testing.T) {
				restore := patchFetchSafetensorsMetadata(func(string) (map[string]any, error) {
					return map[string]any{
						"layer.weight":      map[string]any{"dtype": dtype, "shape": []any{2.0, 128.0}},
						"layer.input_scale": map[string]any{"dtype": dtype, "shape": []any{1.0}},
						"other.biases":      map[string]any{"dtype": dtype, "shape": []any{2.0}},
						"bad":               "skip",
					}, nil
				})
				defer restore()
				got, err := GetModelInfo([]string{"model"}, 512, &types.ModelConfig{QuantizationConfig: json.RawMessage(raw)})
				if err != nil {
					t.Fatal(err)
				}
				if got.TotalParams != 259 || !got.ParamsValid || got.BytesPerParam != GetBytesPerParam(dtype) || got.TensorType != dtype || got.ModelWeightsGB != 0 {
					t.Fatalf("legacy metadata changed: %+v", got)
				}
			})
		}
	}
}

func TestGetModelInfo_MLXCorrectionIsAtomic(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(func(string) (map[string]any, error) {
		return map[string]any{
			"a.weight": map[string]any{"dtype": "U32", "shape": []any{2.0, 16.0}},
			"a.scales": map[string]any{"dtype": "BF16", "shape": []any{2.0, 1.0}},
			"a.biases": map[string]any{"dtype": "BF16", "shape": []any{2.0, 1.0}},
			"b.weight": map[string]any{"dtype": "U32", "shape": []any{2.0, 16.0}},
		}, nil
	})
	defer restore()
	got, err := GetModelInfo([]string{"model"}, 512, &types.ModelConfig{Quantization: json.RawMessage(`{"bits":4,"group_size":128}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalParams != 68 || !got.ParamsValid {
		t.Fatalf("partial correction: %+v", got)
	}
}
