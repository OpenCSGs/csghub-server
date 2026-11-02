package common

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"opencsg.com/csghub-server/common/types"
)

func TestGetModelInfo_AdditionalCorrections(t *testing.T) {
	tensor := func(dtype string, dims ...any) map[string]any { return map[string]any{"dtype": dtype, "shape": dims} }
	for _, tc := range []struct {
		name, config string
		tensors      map[string]any
		want         int64
	}{
		{"mlx mixed mxfp4 affine", `{"quantization":{"bits":4,"group_size":32,"mode":"mxfp4","b":{"bits":8,"group_size":64}}}`, map[string]any{"a.weight": tensor("U32", 2.0, 3.0, 8.0), "a.scales": tensor("U8", 2.0, 3.0, 2.0), "a.bias": tensor("BF16", 2.0, 3.0), "b.weight": tensor("U32", 2.0, 16.0), "b.scales": tensor("BF16", 2.0, 1.0), "b.biases": tensor("BF16", 2.0, 1.0)}, 518},
		{"native mxfp4", `{"quantization_config":{"quant_method":"mxfp4"}}`, map[string]any{"a_blocks": tensor("U8", 2.0, 3.0, 2.0, 16.0), "a_scales": tensor("U8", 2.0, 3.0, 2.0), "a_bias": tensor("BF16", 2.0, 3.0)}, 390},
		{"original mxfp4", `{"quantization_config":{"quant_method":"mxfp4"}}`, map[string]any{"a.blocks": tensor("U8", 2.0, 3.0, 2.0, 16.0), "a.scales": tensor("U8", 2.0, 3.0, 2.0), "a.bias": tensor("BF16", 2.0, 3.0)}, 390},
		{"affine unrelated auxiliary preserved", `{"quantization":{"bits":8,"group_size":64}}`, map[string]any{"a.weight": tensor("U32", 2.0, 16.0), "a.scales": tensor("BF16", 2.0, 1.0), "a.biases": tensor("BF16", 2.0, 1.0), "other.scales": tensor("F32", 3.0)}, 131},
		{"awq", `{"quantization_config":{"quant_method":"awq","version":"gemm","bits":4,"group_size":128,"zero_point":true}}`, map[string]any{"a.qweight": tensor("I32", 128.0, 2.0), "a.qzeros": tensor("I32", 1.0, 2.0), "a.scales": tensor("F16", 1.0, 16.0), "a.bias": tensor("F16", 16.0)}, 2064},
		{"fp8", `{"quantization_config":{"quant_method":"fp8","weight_block_size":[128,128]}}`, map[string]any{"a.weight": tensor("F8_E4M3", 129.0, 128.0), "a.weight_scale_inv": tensor("F32", 2.0, 1.0), "a.bias": tensor("BF16", 129.0)}, 16641},
		{"deepseek fp4", `{"architectures":["DeepseekV4ForCausalLM"],"quantization_config":{"quant_method":"fp8","scale_fmt":"ue8m0","weight_block_size":[128,128]}}`, map[string]any{"a.weight": tensor("I8", 2.0, 64.0), "a.scale": tensor("F8_E8M0", 2.0, 4.0)}, 256},
		{"unrecognized int8", `{"quantization_config":{"quant_method":"fp8","scale_fmt":"ue8m0","weight_block_size":[128,128]}}`, map[string]any{"a.weight": tensor("I8", 2.0, 64.0), "a.scale": tensor("F8_E8M0", 2.0, 4.0)}, 136},
		{"fp8 bad pair atomic fallback", `{"quantization_config":{"quant_method":"fp8","weight_block_size":[128,128]}}`, map[string]any{"a.weight": tensor("F8_E4M3", 128.0, 128.0), "a.weight_scale_inv": tensor("F32", 1.0, 1.0), "b.weight_scale_inv": tensor("F32", 1.0, 1.0)}, 16386},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var config types.ModelConfig
			if err := json.Unmarshal([]byte(tc.config), &config); err != nil {
				t.Fatal(err)
			}
			restore := patchFetchSafetensorsMetadata(func(string) (map[string]any, error) { return tc.tensors, nil })
			defer restore()
			got, err := GetModelInfo([]string{"model"}, 512, &config)
			if err != nil {
				t.Fatal(err)
			}
			if !got.ParamsValid || got.TotalParams != tc.want {
				t.Fatalf("got %+v, want %d", got, tc.want)
			}
			duplicate, err := GetModelInfo([]string{"model", "duplicate"}, 512, &config)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := GetModelInfo([]string{"model", "duplicate"}, 512, nil)
			if err != nil {
				t.Fatal(err)
			}
			if duplicate.TotalParams != legacy.TotalParams || duplicate.ParamsValid != legacy.ParamsValid {
				t.Fatalf("duplicate changed legacy result: %+v vs %+v", duplicate, legacy)
			}
		})
	}
}

// Compact real shape/multiplicity fixtures: 642 OpenCSG shards plus GPT-OSS
// Hugging Face headers, checked on 2026-10-14. No tensor values or execution.
func TestGetModelInfo_RealQuantizedModelShapes(t *testing.T) {
	var fixtures []struct {
		Model  string
		Config types.ModelConfig
		Want   int64
		Legacy int64
		Groups []struct {
			Copies  int
			Prefix  string
			Tensors []struct {
				Suffix, Dtype string
				Shape         []any
			}
		}
	}
	data, err := os.ReadFile("testdata/quantized_model_shapes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Model, func(t *testing.T) {
			shards := []map[string]any{{}, {}}
			for group, g := range fixture.Groups {
				for copy := 0; copy < g.Copies; copy++ {
					module := fmt.Sprintf("module%d_%d%s", group, copy, g.Prefix)
					for i, tensor := range g.Tensors {
						shards[i%2][module+tensor.Suffix] = map[string]any{"dtype": tensor.Dtype, "shape": tensor.Shape}
					}
				}
			}
			restore := patchFetchSafetensorsMetadata(func(file string) (map[string]any, error) {
				if file == "weights" {
					return shards[0], nil
				}
				return shards[1], nil
			})
			defer restore()
			got, err := GetModelInfo([]string{"weights", "aux"}, 512, &fixture.Config)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := GetModelInfo([]string{"weights", "aux"}, 512, nil)
			if err != nil {
				t.Fatal(err)
			}
			if legacy.TotalParams != fixture.Legacy || legacy.ModelWeightsGB != got.ModelWeightsGB {
				t.Fatalf("storage or original count changed: %+v vs %+v", got, legacy)
			}
			if !got.ParamsValid || got.TotalParams != fixture.Want {
				t.Fatalf("got %+v, want %d", got, fixture.Want)
			}
		})
	}
}

func TestGetModelInfo_CorrectionFallbacks(t *testing.T) {
	for _, tc := range []struct{ name, config, header string }{
		{"AWQ missing zeros", `{"quantization_config":{"quant_method":"awq","version":"gemm","bits":4,"group_size":128,"zero_point":true}}`, `{"a.qweight":{"dtype":"I32","shape":[128,2]},"a.scales":{"dtype":"F16","shape":[1,16]}}`},
		{"AWQ wrong layout", `{"quantization_config":{"quant_method":"awq","version":"gemm","bits":4,"group_size":128,"zero_point":true}}`, `{"a.qweight":{"dtype":"I32","shape":[128,2]},"a.scales":{"dtype":"F16","shape":[16,1]},"a.qzeros":{"dtype":"I32","shape":[1,2]}}`},
		{"unsupported AWQ version", `{"quantization_config":{"quant_method":"awq","version":"gemv","bits":4,"group_size":128,"zero_point":true}}`, `{"a.qweight":{"dtype":"I32","shape":[128,2]},"a.scales":{"dtype":"F16","shape":[1,16]},"a.qzeros":{"dtype":"I32","shape":[1,2]}}`},
		{"FP8 missing scale", `{"quantization_config":{"quant_method":"fp8","weight_block_size":[128,128]}}`, `{"a.weight":{"dtype":"F8_E4M3","shape":[128,128]}}`},
		{"FP8 wrong block scale", `{"quantization_config":{"quant_method":"fp8","weight_block_size":[128,128]}}`, `{"a.weight":{"dtype":"F8_E4M3","shape":[128,128]},"a.weight_scale_inv":{"dtype":"F32","shape":[2,1]}}`},
		{"FP8 unknown block size", `{"quantization_config":{"quant_method":"fp8","weight_block_size":[0,128]}}`, `{"a.weight":{"dtype":"F8_E4M3","shape":[128,128]},"a.weight_scale_inv":{"dtype":"F32","shape":[1,1]}}`},
		{"FP4 missing scale", `{"architectures":["DeepseekV41ForCausalLM"],"quantization_config":{"quant_method":"fp8","scale_fmt":"ue8m0","weight_block_size":[32,32]}}`, `{"a.weight":{"dtype":"I8","shape":[2,64]}}`},
		{"FP4 wrong scale dtype", `{"architectures":["DeepseekV41ForCausalLM"],"quantization_config":{"quant_method":"fp8","scale_fmt":"ue8m0","weight_block_size":[32,32]}}`, `{"a.weight":{"dtype":"I8","shape":[2,64]},"a.scale":{"dtype":"F32","shape":[2,4]}}`},
		{"FP4 wrong packing", `{"architectures":["DeepseekV41ForCausalLM"],"quantization_config":{"quant_method":"fp8","scale_fmt":"ue8m0","weight_block_size":[32,32]}}`, `{"a.weight":{"dtype":"I8","shape":[2,65]},"a.scale":{"dtype":"F8_E8M0","shape":[2,4]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg types.ModelConfig
			if err := json.Unmarshal([]byte(tc.config), &cfg); err != nil {
				t.Fatal(err)
			}
			restore := patchFetchSafetensorsMetadata(func(string) (map[string]any, error) {
				var h map[string]any
				err := json.Unmarshal([]byte(tc.header), &h)
				return h, err
			})
			defer restore()
			legacy, err := GetModelInfo([]string{"model"}, 512, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := GetModelInfo([]string{"model"}, 512, &cfg)
			if err != nil {
				t.Fatal(err)
			}
			if got.TotalParams != legacy.TotalParams || got.ParamsValid != legacy.ParamsValid || got.ModelWeightsGB != legacy.ModelWeightsGB {
				t.Fatalf("fallback changed old result: got %+v legacy %+v", got, legacy)
			}
		})
	}
}

func TestApplyParameterReplacements_Overflow(t *testing.T) {
	tensors := map[string]safetensorInfo{"a": {elements: math.MaxInt64 - 1}, "b": {elements: 1}}
	if _, ok := applyParameterReplacements(math.MaxInt64, tensors, map[string]int64{"b": 2}); ok {
		t.Fatal("overflow accepted")
	}
	if _, ok := applyParameterReplacements(0, tensors, map[string]int64{"b": 0}); ok {
		t.Fatal("inconsistent legacy count accepted")
	}
}

func TestGetModelInfo_MXFP4Fallbacks(t *testing.T) {
	for _, format := range []string{"mlx", "native"} {
		for _, invalid := range []string{"missing scale", "wrong scale dtype", "wrong shape", "wrong packing", "wrong weight dtype", "extra aux", "unknown config"} {
			t.Run(format+"/"+invalid, func(t *testing.T) {
				var cfg types.ModelConfig
				var header string
				var scaleName, weightName string
				if format == "mlx" {
					cfg.Quantization = json.RawMessage(`{"bits":4,"group_size":32,"mode":"mxfp4"}`)
					header = `{"a.weight":{"dtype":"U32","shape":[2,8]},"a.scales":{"dtype":"U8","shape":[2,2]},"b.weight":{"dtype":"U32","shape":[2,8]},"b.scales":{"dtype":"U8","shape":[2,2]}}`
					scaleName, weightName = "a.scales", "a.weight"
				} else {
					cfg.QuantizationConfig = json.RawMessage(`{"quant_method":"mxfp4"}`)
					header = `{"a_blocks":{"dtype":"U8","shape":[2,2,16]},"a_scales":{"dtype":"U8","shape":[2,2]},"b_blocks":{"dtype":"U8","shape":[2,2,16]},"b_scales":{"dtype":"U8","shape":[2,2]}}`
					scaleName, weightName = "a_scales", "a_blocks"
				}
				var tensors map[string]any
				if err := json.Unmarshal([]byte(header), &tensors); err != nil {
					t.Fatal(err)
				}
				scale := tensors[scaleName].(map[string]any)
				weight := tensors[weightName].(map[string]any)
				switch invalid {
				case "missing scale":
					delete(tensors, scaleName)
				case "wrong scale dtype":
					scale["dtype"] = "BF16"
				case "wrong shape":
					scale["shape"] = []any{3.0, 2.0}
				case "wrong packing":
					weight["shape"] = []any{2.0, 2.0, 15.0}
				case "wrong weight dtype":
					weight["dtype"] = "I32"
				case "extra aux":
					if format == "mlx" {
						tensors["a.biases"] = scale
					} else {
						tensors["orphan_scales"] = scale
					}
				case "unknown config":
					cfg = types.ModelConfig{}
				}
				restore := patchFetchSafetensorsMetadata(func(string) (map[string]any, error) { return tensors, nil })
				defer restore()
				legacy, err := GetModelInfo([]string{"model"}, 512, nil)
				if err != nil {
					t.Fatal(err)
				}
				got, err := GetModelInfo([]string{"model"}, 512, &cfg)
				if err != nil {
					t.Fatal(err)
				}
				if got.TotalParams != legacy.TotalParams || got.ParamsValid != legacy.ParamsValid || got.ModelWeightsGB != legacy.ModelWeightsGB {
					t.Fatalf("fallback changed old result: got %+v legacy %+v", got, legacy)
				}
			})
		}
	}
}
