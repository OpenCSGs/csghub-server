package common

import (
	"encoding/json"
	"fmt"
	"opencsg.com/csghub-server/common/types"
	"testing"
)

func TestGetModelInfo_MLXQuantization(t *testing.T) {
	for _, field := range []string{"quantization", "quantization_config"} {
		t.Run(field, func(t *testing.T) {
			var config types.ModelConfig
			if err := json.Unmarshal([]byte(`{"`+field+`":{"bits":6,"group_size":128}}`), &config); err != nil {
				t.Fatal(err)
			}
			restore := patchFetchSafetensorsMetadata(func(file string) (map[string]any, error) {
				if file == "weights" {
					return map[string]any{
						"model.embed_tokens.weight": map[string]any{"dtype": "U32", "shape": []any{151936.0, 960.0}},
						"model.norm.weight":         map[string]any{"dtype": "BF16", "shape": []any{5120.0}},
						"projection.bias":           map[string]any{"dtype": "BF16", "shape": []any{5120.0}},
					}, nil
				}
				return map[string]any{
					"model.embed_tokens.scales": map[string]any{"dtype": "BF16", "shape": []any{151936.0, 40.0}},
					"model.embed_tokens.biases": map[string]any{"dtype": "BF16", "shape": []any{151936.0, 40.0}},
				}, nil
			})
			defer restore()
			got, err := GetModelInfo([]string{"weights", "auxiliary"}, 512, &config)
			if err != nil {
				t.Fatal(err)
			}
			const want int64 = 151936*5120 + 5120 + 5120
			if !got.ParamsValid || got.TotalParams != want {
				t.Fatalf("got %+v, want %d valid parameters", got, want)
			}
			// The packed checkpoint is below 1 GiB, whereas expanded BF16 weights exceed it.
			if got.ModelWeightsGB != 0 {
				t.Fatalf("storage size inflated by logical count: %v", got.ModelWeightsGB)
			}
		})
	}
}

func TestGetModelInfo_QuantizationValidation(t *testing.T) {
	cases := []struct {
		name, config            string
		weightShape, scaleShape []any
		dtype                   string
		missingBias             bool
		want                    int64
		valid                   bool
	}{
		{name: "mlx1", config: `{"quantization":{"bits":1,"group_size":128}}`, weightShape: []any{2.0, 4.0}, want: 256, valid: true},
		{name: "mlx7", config: `{"quantization":{"bits":7,"group_size":128}}`, weightShape: []any{2.0, 28.0}},
		{name: "invalid group", config: `{"quantization":{"bits":4,"group_size":256}}`, weightShape: []any{2.0, 32.0}},
		{name: "mlx2", config: `{"quantization":{"bits":2,"group_size":128}}`, weightShape: []any{2.0, 8.0}, want: 256, valid: true},
		{name: "mlx3", config: `{"quantization":{"bits":3,"group_size":128}}`, weightShape: []any{2.0, 12.0}, want: 256, valid: true},
		{name: "mlx5", config: `{"quantization":{"bits":5,"group_size":128}}`, weightShape: []any{2.0, 20.0}, want: 256, valid: true},
		{name: "mlx6", config: `{"quantization":{"bits":6,"group_size":128}}`, weightShape: []any{2.0, 24.0}, want: 256, valid: true},
		{name: "mlx8", config: `{"quantization":{"bits":8,"group_size":128}}`, weightShape: []any{2.0, 32.0}, want: 256, valid: true},
		{name: "mlx4", config: `{"quantization":{"bits":4,"group_size":128}}`, want: 256, valid: true},
		{name: "per layer only", config: `{"quantization":{"layer":{"bits":4,"group_size":128}}}`, want: 256, valid: true},
		{name: "incomplete override", config: `{"quantization":{"bits":6,"group_size":128,"layer":{"bits":4}}}`},
		{name: "null layer", config: `{"quantization":{"bits":4,"group_size":128,"layer":null}}`},
		{name: "per layer override", config: `{"quantization":{"bits":6,"group_size":128,"layer":{"bits":4,"group_size":128}}}`, want: 256, valid: true},
		{name: "unknown method", config: `{"quantization_config":{"quant_method":"gptq","bits":4,"group_size":128}}`},
		{name: "unknown mode", config: `{"quantization":{"bits":4,"group_size":128,"mode":"mxfp4"}}`},
		{name: "wrong bits", config: `{"quantization":{"bits":6,"group_size":128}}`},
		{name: "missing group size", config: `{"quantization":{"bits":4}}`},
		{name: "missing biases", config: `{"quantization":{"bits":4,"group_size":128}}`, missingBias: true},
		{name: "transposed scales", config: `{"quantization":{"bits":4,"group_size":128}}`, scaleShape: []any{1.0, 2.0}},
		{name: "wrong dtype", config: `{"quantization":{"bits":4,"group_size":128}}`, dtype: "I32"},
		{name: "disabled layer", config: `{"quantization":{"bits":4,"group_size":128,"layer":false}}`},
		{name: "unsupported layer", config: `{"quantization":{"bits":4,"group_size":128,"layer":{"mode":"mxfp4"}}}`},
		{name: "malformed declaration", config: `{"quantization":true}`},
		{name: "fractional shape", weightShape: []any{2.0, 16.5}},
		{name: "negative shape", weightShape: []any{-2.0, 16.0}},
		{name: "overflow", weightShape: []any{float64(1 << 62), 16.0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var config types.ModelConfig
			if tc.config != "" {
				if err := json.Unmarshal([]byte(tc.config), &config); err != nil {
					t.Fatal(err)
				}
			}
			if tc.weightShape == nil {
				tc.weightShape = []any{2.0, 16.0}
			}
			if tc.scaleShape == nil {
				tc.scaleShape = []any{2.0, 1.0}
			}
			if tc.dtype == "" {
				tc.dtype = "U32"
			}
			restore := patchFetchSafetensorsMetadata(func(string) (map[string]any, error) {
				h := map[string]any{
					"layer.weight": map[string]any{"dtype": tc.dtype, "shape": tc.weightShape},
					"layer.scales": map[string]any{"dtype": "BF16", "shape": tc.scaleShape},
				}
				if !tc.missingBias {
					h["layer.biases"] = map[string]any{"dtype": "BF16", "shape": []any{2.0, 1.0}}
				}
				return h, nil
			})
			defer restore()
			got, err := GetModelInfo([]string{"model"}, 512, &config)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.valid {
				// Invalid or unsupported corrections preserve the old shape count.
				tc.want = calculateTensorParamsForTest(tc.weightShape) + calculateTensorParamsForTest(tc.scaleShape)
				if !tc.missingBias {
					tc.want += 2
				}
				tc.valid = true
			}
			if got.ParamsValid != tc.valid || got.TotalParams != tc.want {
				t.Fatalf("got %+v; want count=%d valid=%v", got, tc.want, tc.valid)
			}
		})
	}
}

// Shapes and multiplicities from Qwen/Qwen3-32B-MLX-6bit at
// acfa81570324c411687c879e1807295f7542f5bb. Keep only header information;
// this regression requires neither remote access nor the 25 GB checkpoint.
func TestGetModelInfo_Qwen3MLX6Bit(t *testing.T) {
	shards := make([]map[string]any, 5)
	files := make([]string, len(shards))
	for i := range shards {
		shards[i] = make(map[string]any)
		files[i] = fmt.Sprint(i)
	}
	tensorIndex := 0
	add := func(name, dtype string, shape ...any) {
		// Distribute associated tensors across shards to exercise checkpoint-wide pairing.
		index := tensorIndex % len(shards)
		tensorIndex++
		shards[index][name] = map[string]any{"dtype": dtype, "shape": shape}
	}
	for group, spec := range []struct{ rows, packedWidth, groups, copies int }{
		{151936, 960, 40, 2}, {25600, 960, 40, 128}, {1024, 960, 40, 128},
		{5120, 4800, 200, 64}, {5120, 1536, 64, 64}, {8192, 960, 40, 64},
	} {
		for i := 0; i < spec.copies; i++ {
			name := fmt.Sprintf("group%d.layer%d", group, i)
			add(name+".weight", "U32", float64(spec.rows), float64(spec.packedWidth))
			add(name+".scales", "BF16", float64(spec.rows), float64(spec.groups))
			add(name+".biases", "BF16", float64(spec.rows), float64(spec.groups))
		}
	}
	for i := 0; i < 129; i++ {
		add(fmt.Sprintf("norm%d.weight", i), "BF16", 5120.0)
	}
	for i := 0; i < 128; i++ {
		add(fmt.Sprintf("qknorm%d.weight", i), "BF16", 128.0)
	}
	restore := patchFetchSafetensorsMetadata(func(file string) (map[string]any, error) {
		for i, name := range files {
			if file == name {
				return shards[i], nil
			}
		}
		return nil, fmt.Errorf("unexpected file %s", file)
	})
	defer restore()
	config := &types.ModelConfig{Quantization: json.RawMessage(`{"bits":6,"group_size":128}`)}
	got, err := GetModelInfo(files, 512, config)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ParamsValid || got.TotalParams != 32762123264 || got.ParamsBillions != float32(32.76) {
		t.Fatalf("Qwen3-32B MLX count: %+v", got)
	}
	if got.ModelWeightsGB != 23 {
		t.Fatalf("storage size changed: %v", got.ModelWeightsGB)
	}
}

func calculateTensorParamsForTest(shape []any) int64 {
	n := int64(1)
	for _, dim := range shape {
		n *= int64(int(dim.(float64)))
	}
	return n
}
