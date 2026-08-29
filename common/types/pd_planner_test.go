package types

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanPD_DeepSeekV3_H200(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 671, NonMoEParamsB: 37, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, prefill.TP, 8)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("DeepSeek-V3 H200: Prefill TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB kv=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU, prefill.RemainingVRAMForKV)
	t.Logf("DeepSeek-V3 H200: Decode TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB kv=%.2fGB",
		decode.TP, decode.EP, decode.TotalGPUs, decode.LWSSize, decode.WeightMemPerGPU, decode.RemainingVRAMForKV)
}

func TestPlanPD_DeepSeekV3_A800_MultiNode(t *testing.T) {
	// DeepSeek-V3 fp8 (671B): TP=8 weightMem=96.5GB > 68GB budget on 80GB GPU (single node).
	// With multi-node TP=16: (37/16 + 634/16)*1.0*1.15 = 48.2GB < 68GB — fits on 2 nodes.
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 671, NonMoEParamsB: 37, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 16, prefill.TP)
	require.Equal(t, 16, prefill.TotalGPUs)
	require.Equal(t, 2, prefill.LWSSize) // 2 nodes of 8 GPUs
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Equal(t, 16, decode.TP)
	require.Equal(t, 16, decode.TotalGPUs)

	t.Logf("DeepSeek-V3 A800 multi-node: Prefill TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU)
	t.Logf("DeepSeek-V3 A800 multi-node: Decode TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB",
		decode.TP, decode.EP, decode.TotalGPUs, decode.LWSSize, decode.WeightMemPerGPU)
}

func TestPlanPD_DeepSeekV4Flash_H200(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 284, NonMoEParamsB: 13, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("DeepSeek-V4-Flash H200: Prefill TP=%d EP=%d GPUs=%d", prefill.TP, prefill.EP, prefill.TotalGPUs)
	t.Logf("DeepSeek-V4-Flash H200: Decode TP=%d EP=%d GPUs=%d", decode.TP, decode.EP, decode.TotalGPUs)
}

func TestPlanPD_DeepSeekV4Pro_H200(t *testing.T) {
	// DeepSeek-V4-Pro fp8 (1600B): TP=8 too large for single node (230GB > 119.85GB).
	// Multi-node TP=16: (49/16 + 1551/16)*1.0*1.15 = 115.0GB < 119.85GB — fits on 2 nodes.
	// 384 % 16 = 0, so EP=TP=16 is valid.
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 1600, NonMoEParamsB: 49, TotalExperts: 384, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 16, prefill.TotalGPUs)
	require.Equal(t, 2, prefill.LWSSize) // 2 nodes of 8 GPUs
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Equal(t, 16, decode.TotalGPUs)

	t.Logf("DeepSeek-V4-Pro H200: Prefill TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU)
}

func TestPlanPD_GLM51_H200(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 744, NonMoEParamsB: 40, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("GLM-5.1 H200: Prefill TP=%d EP=%d GPUs=%d", prefill.TP, prefill.EP, prefill.TotalGPUs)
	t.Logf("GLM-5.1 H200: Decode TP=%d EP=%d GPUs=%d", decode.TP, decode.EP, decode.TotalGPUs)
}

func TestPlanPD_Qwen3_235B_BF16_A800(t *testing.T) {
	// Qwen3-235B-A22B bf16: the model that triggered the original bug report.
	// TP=8 weightMem=67.6GB < 68GB budget (80*0.85) — fits on 8×80GB GPUs.
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 235, NonMoEParamsB: 18.92, TotalExperts: 128, Precision: "bf16"},
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0, "prefill should have KV headroom")
	require.Greater(t, decode.RemainingVRAMForKV, 0.0, "decode should have KV headroom")
	require.Equal(t, 8, prefill.TP)
	require.Equal(t, 8, prefill.EP)
	require.Equal(t, 8, prefill.TotalGPUs)
	require.Equal(t, 8, decode.TP)
	require.Equal(t, 8, decode.TotalGPUs)

	t.Logf("Qwen3-235B bf16 A800: Prefill TP=%d EP=%d GPUs=%d weight=%.2fGB kv=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.WeightMemPerGPU, prefill.RemainingVRAMForKV)
	t.Logf("Qwen3-235B bf16 A800: Decode TP=%d EP=%d GPUs=%d weight=%.2fGB kv=%.2fGB",
		decode.TP, decode.EP, decode.TotalGPUs, decode.WeightMemPerGPU, decode.RemainingVRAMForKV)
}

func TestPlanPD_GLM52_H200(t *testing.T) {
	// GLM-5.2 fp8 (1000B): TP=8 weightMem=143.8GB > 119.85GB on H200 single node.
	// Multi-node TP=16: (50/16 + 950/16)*1.0*1.15 = 71.9GB < 119.85GB — fits on 2 nodes.
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 1000, NonMoEParamsB: 50, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 16, prefill.TotalGPUs)
	require.Equal(t, 2, prefill.LWSSize)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Equal(t, 16, decode.TotalGPUs)

	t.Logf("GLM-5.2 H200: Prefill TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU)
}

func TestPlanPD_K3_2_8T_MultiNode(t *testing.T) {
	// K3 (2.8T) fp8, 256 experts on H200 (141GB).
	// TP=8:  (50/8 + 2750/8)*1.15 = 402.5GB  — way too large for single node.
	// TP=16: (50/16 + 2750/16)*1.15 = 201.3GB — still too large.
	// TP=32: (50/32 + 2750/32)*1.15 = 100.6GB < 119.85GB — fits on 4 nodes!
	// 256 % 32 = 0, so EP=TP=32 is valid.
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 2800, NonMoEParamsB: 50, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 32, prefill.TP)
	require.Equal(t, 32, prefill.EP)
	require.Equal(t, 32, prefill.TotalGPUs)
	require.Equal(t, 4, prefill.LWSSize) // 4 nodes of 8 GPUs
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Equal(t, 32, decode.TP)
	require.Equal(t, 32, decode.TotalGPUs)
	require.Equal(t, 4, decode.LWSSize)

	t.Logf("K3 2.8T H200: Prefill TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB kv=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU, prefill.RemainingVRAMForKV)
	t.Logf("K3 2.8T H200: Decode TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB kv=%.2fGB",
		decode.TP, decode.EP, decode.TotalGPUs, decode.LWSSize, decode.WeightMemPerGPU, decode.RemainingVRAMForKV)
}

func TestPlanPD_GPT120B_A800(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 120, NonMoEParamsB: 120, TotalExperts: 0, Precision: "bf16"},
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 1, prefill.EP)
	require.Equal(t, 1, decode.EP)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("GPT-120B A800: Prefill TP=%d EP=%d GPUs=%d", prefill.TP, prefill.EP, prefill.TotalGPUs)
	t.Logf("GPT-120B A800: Decode TP=%d EP=%d GPUs=%d", decode.TP, decode.EP, decode.TotalGPUs)
}

func TestPlanPD_GPT120B_H200(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 120, NonMoEParamsB: 120, TotalExperts: 0, Precision: "bf16"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 1, prefill.EP)
	require.Equal(t, 1, decode.EP)

	t.Logf("GPT-120B H200: Prefill TP=%d EP=%d GPUs=%d", prefill.TP, prefill.EP, prefill.TotalGPUs)
	t.Logf("GPT-120B H200: Decode TP=%d EP=%d GPUs=%d", decode.TP, decode.EP, decode.TotalGPUs)
}

func TestPlanPD_Qwen3_30B_A3B_A800(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 30, NonMoEParamsB: 3, TotalExperts: 128, Precision: "bf16"},
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("Qwen3-30B-A3B A800: Prefill TP=%d EP=%d GPUs=%d", prefill.TP, prefill.EP, prefill.TotalGPUs)
	t.Logf("Qwen3-30B-A3B A800: Decode TP=%d EP=%d GPUs=%d", decode.TP, decode.EP, decode.TotalGPUs)
}

func TestPlanPD_Qwen1_5MoE_A800(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 14.3, NonMoEParamsB: 2.5, TotalExperts: 60, Precision: "bf16"},
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("Qwen1.5-MoE-A2.7B A800: Prefill TP=%d EP=%d GPUs=%d", prefill.TP, prefill.EP, prefill.TotalGPUs)
	t.Logf("Qwen1.5-MoE-A2.7B A800: Decode TP=%d EP=%d GPUs=%d", decode.TP, decode.EP, decode.TotalGPUs)
}

func TestPlanPD_DenseModelEPAlwaysOne(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 72, NonMoEParamsB: 72, TotalExperts: 0, Precision: "bf16"},
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 1, prefill.EP)
	require.Equal(t, 1, decode.EP)
}

func TestPlanPD_WeightMemCalculation(t *testing.T) {
	// Dense model: expertParamsB=0, so formula is (NonMoE/TP) * bParam * overhead.
	// (72/2 + 0/1) * 2.0 * 1.15 = 36 * 2.3 = 82.8
	weight := calcWeightMemPerGPU(72, 0, 2, 1, 2.0)
	require.InDelta(t, 82.8, weight, 0.01)
}

func TestPlanPD_WeightMemCalculation_MoE(t *testing.T) {
	// MoE model: NonMoE sharded by TP, Expert sharded by EP (independent, not nested).
	// NonMoE=20, Expert=200, TP=4, EP=4, bParam=2.0, overhead=1.15
	// (20/4 + 200/4) * 2.0 * 1.15 = (5 + 50) * 2.3 = 55 * 2.3 = 126.5
	weight := calcWeightMemPerGPU(20, 200, 4, 4, 2.0)
	require.InDelta(t, 126.5, weight, 0.01)
}

func TestPlanPD_SmallModelSingleNode(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 7, NonMoEParamsB: 7, TotalExperts: 0, Precision: "bf16"},
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 1, prefill.TotalGPUs)
	require.Equal(t, 1, prefill.TP)
	require.Equal(t, 1, prefill.LWSSize)
	require.Equal(t, 1, decode.TotalGPUs)
	require.Equal(t, 1, decode.TP)
}

func TestPlanPD_DefaultKVRatios(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model:          PDModelSpec{TotalParamsB: 72, NonMoEParamsB: 72, TotalExperts: 0, Precision: "bf16"},
		GPU:            PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
		KVRatioPrefill: 0,
		KVRatioDecode:  0,
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)
}

func TestPlanPD_LWSSizeCalculation(t *testing.T) {
	_, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 284, NonMoEParamsB: 13, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	if decode.TotalGPUs > 8 {
		expectedSize := int(math.Ceil(float64(decode.TotalGPUs) / 8.0))
		require.Equal(t, expectedSize, decode.LWSSize)
		require.Equal(t, 8, decode.GPUsPerPod)
	}
}

func TestPlanPD_FullMatrix(t *testing.T) {
	models := map[string]PDModelSpec{
		"DeepSeek-V3":          {TotalParamsB: 671, NonMoEParamsB: 37, TotalExperts: 256, Precision: "fp8"},
		"DeepSeek-V4-Flash":    {TotalParamsB: 284, NonMoEParamsB: 13, TotalExperts: 256, Precision: "fp8"},
		"GLM-5.1":              {TotalParamsB: 744, NonMoEParamsB: 40, TotalExperts: 256, Precision: "fp8"},
		"Qwen3-235B-A22B":      {TotalParamsB: 235, NonMoEParamsB: 18.92, TotalExperts: 128, Precision: "bf16"},
		"Qwen3-235B-A22B-FP8":  {TotalParamsB: 235, NonMoEParamsB: 18.92, TotalExperts: 128, Precision: "fp8"},
		"GPT-120B":             {TotalParamsB: 120, NonMoEParamsB: 120, TotalExperts: 0, Precision: "bf16"},
		"Qwen3-30B-A3B":        {TotalParamsB: 30, NonMoEParamsB: 3, TotalExperts: 128, Precision: "bf16"},
		"Qwen1.5-MoE":          {TotalParamsB: 14.3, NonMoEParamsB: 2.5, TotalExperts: 60, Precision: "bf16"},
		"K3-2.8T":              {TotalParamsB: 2800, NonMoEParamsB: 50, TotalExperts: 256, Precision: "fp8"},
	}

	// All combinations are now feasible thanks to multi-node TP support
	// with MaxTotalGPUs=1024 (up to 128 nodes of 8 GPUs).
	gpus := map[string]PDGPUConfig{
		"A800": {VRAMGB: 80, GPUsPerNode: 8},
		"H200": {VRAMGB: 141, GPUsPerNode: 8},
	}

	for modelName, spec := range models {
		for gpuName, gpu := range gpus {
			key := modelName + "_" + gpuName
			t.Run(key, func(t *testing.T) {
				prefill, decode, err := PlanPD(PDPlanInput{Model: spec, GPU: gpu})
				require.NoError(t, err, "PlanPD should succeed for %s on %s", modelName, gpuName)
				require.Greater(t, prefill.RemainingVRAMForKV, 0.0, "prefill should have KV headroom")
				require.Greater(t, decode.RemainingVRAMForKV, 0.0, "decode should have KV headroom")

				t.Logf("%-25s %-5s | Prefill: TP=%2d EP=%3d GPUs=%2d Size=%d weight=%.1fGB | Decode: TP=%2d EP=%3d GPUs=%2d Size=%d weight=%.1fGB",
					modelName, gpuName,
					prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU,
					decode.TP, decode.EP, decode.TotalGPUs, decode.LWSSize, decode.WeightMemPerGPU)
			})
		}
	}
}

func TestPlanPDRecommendation_Qwen3_235B(t *testing.T) {
	// Qwen3-235B-A22B bf16 with config.json values: hidden_size=4096, num_hidden_layers=94.
	// NonMoE = 94 * 12 * 4096^2 / 1e9 = 18.92B.
	// TP=8 weightMem = (18.92/8 + 216.08/8) * 2.0 * 1.15 = 67.6GB < 68GB budget.
	rec, err := PlanPDRecommendation("AIWizards/Qwen3-235B-A22B", 235, 128, 8, "bf16", 4096, 94, 460)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 235.0, rec.TotalParamsB)
	require.Equal(t, 128, rec.TotalExperts)
	require.Equal(t, 8, rec.ActiveExperts)
	require.Equal(t, "bf16", rec.Precision)
	require.Equal(t, 460.0, rec.MinInferenceVRAMGB)
	require.Greater(t, rec.Prefill.TotalGPUs, 0)
	require.Greater(t, rec.Decode.TotalGPUs, 0)
	// TotalVRAMGB = MinInferenceVRAMGB * Pods
	require.Equal(t, 460.0*float64(rec.Prefill.Pods), rec.Prefill.TotalVRAMGB)
	require.Equal(t, 460.0*float64(rec.Decode.Pods), rec.Decode.TotalVRAMGB)
	// Should use TP=8 (the only config that fits on 80GB)
	require.Equal(t, 8, rec.Prefill.TP)
	require.Equal(t, 8, rec.Prefill.EP)
	require.Equal(t, 8, rec.Prefill.TotalGPUs)

	t.Logf("Qwen3-235B-A22B: Prefill TP=%d EP=%d GPUs=%d Pods=%d VRAM=%.0f | Decode TP=%d EP=%d GPUs=%d Pods=%d VRAM=%.0f",
		rec.Prefill.TP, rec.Prefill.EP, rec.Prefill.TotalGPUs, rec.Prefill.Pods, rec.Prefill.TotalVRAMGB,
		rec.Decode.TP, rec.Decode.EP, rec.Decode.TotalGPUs, rec.Decode.Pods, rec.Decode.TotalVRAMGB)
}

func TestPlanPDRecommendation_MinInferenceVRAMZero(t *testing.T) {
	// When minInferenceVRAMGB is 0 (metadata.mini_gpu_memory_gb not populated),
	// keep it as 0 so the missing value is visible. PD planning still succeeds;
	// only the VRAM fields are 0 until the value is resolved from TOML/metadata.
	rec, err := PlanPDRecommendation("qwen3-235b", 235, 128, 8, "bf16", 4096, 94, 0)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 0.0, rec.MinInferenceVRAMGB)
	require.Equal(t, 0.0, rec.Prefill.TotalVRAMGB)
	require.Equal(t, 0.0, rec.Decode.TotalVRAMGB)
	require.Greater(t, rec.Prefill.TotalGPUs, 0)
	require.Greater(t, rec.Decode.TotalGPUs, 0)
}

func TestPlanPDRecommendation_MinInferenceVRAMNegative(t *testing.T) {
	// Negative values should be clamped to 0.
	rec, err := PlanPDRecommendation("qwen3-235b", 235, 128, 8, "bf16", 4096, 94, -10)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 0.0, rec.MinInferenceVRAMGB)
	require.Equal(t, 0.0, rec.Prefill.TotalVRAMGB)
	require.Equal(t, 0.0, rec.Decode.TotalVRAMGB)
}

func TestPlanPDRecommendation_Qwen3_30B_A3B(t *testing.T) {
	rec, err := PlanPDRecommendation("qwen3-30b-a3b", 30, 128, 8, "bf16", 2048, 48, 0)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 30.0, rec.TotalParamsB)
	require.Equal(t, 128, rec.TotalExperts)
}

func TestPlanPDRecommendation_UnknownModelMoE(t *testing.T) {
	// Unknown MoE model with expert info from config.json
	rec, err := PlanPDRecommendation("custom/moe-model", 200, 64, 8, "bf16", 4096, 32, 0)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 200.0, rec.TotalParamsB)
	require.Equal(t, 64, rec.TotalExperts)
	require.Equal(t, 8, rec.ActiveExperts)
	// NonMoEParamsB should be estimated from hidden_size and num_hidden_layers
	require.Greater(t, rec.NonMoEParamsB, 0.0)
	require.Less(t, rec.NonMoEParamsB, rec.TotalParamsB)
}

func TestPlanPDRecommendation_DenseModel(t *testing.T) {
	rec, err := PlanPDRecommendation("custom-dense-70b", 70, 0, 0, "bf16", 0, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 70.0, rec.TotalParamsB)
	require.Equal(t, 0, rec.TotalExperts)
	require.Equal(t, 70.0, rec.NonMoEParamsB)
}

func TestPlanPDRecommendation_ZeroParams(t *testing.T) {
	_, err := PlanPDRecommendation("test", 0, 256, 8, "fp8", 0, 0, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "total params must be positive")
}

func TestPlanPDRecommendation_NegativeParams(t *testing.T) {
	_, err := PlanPDRecommendation("test", -10, 256, 8, "fp8", 0, 0, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "total params must be positive")
}

func TestPlanPDRecommendation_PrecisionOverride(t *testing.T) {
	// Known model with explicit precision override
	// Use a smaller model that fits in 80GB with bf16
	rec, err := PlanPDRecommendation("qwen3-30b-a3b", 30, 128, 8, "bf16", 2048, 48, 0)
	require.NoError(t, err)
	require.Equal(t, "bf16", rec.Precision)
}

func TestPlanPDRecommendation_DPField(t *testing.T) {
	rec, err := PlanPDRecommendation("qwen3-235b", 235, 128, 8, "bf16", 4096, 94, 0)
	require.NoError(t, err)
	// DP should default to 1 for all recommendations
	require.Equal(t, 1, rec.Prefill.DP)
	require.Equal(t, 1, rec.Decode.DP)
	// TotalGPUs should equal TP (DP=1, EP follows TP and does not add extra GPUs)
	require.Equal(t, rec.Prefill.TP, rec.Prefill.TotalGPUs)
	require.Equal(t, rec.Decode.TP, rec.Decode.TotalGPUs)
}

func TestPlanPD_TP_Equals_EP(t *testing.T) {
	// For MoE models, EP must equal TP (EP follows TP convention).
	// For dense models, EP must be 1.
	moeSpecs := []PDModelSpec{
		{TotalParamsB: 235, NonMoEParamsB: 18.92, TotalExperts: 128, Precision: "bf16"},
		{TotalParamsB: 30, NonMoEParamsB: 3, TotalExperts: 128, Precision: "bf16"},
		{TotalParamsB: 284, NonMoEParamsB: 13, TotalExperts: 256, Precision: "fp8"},
	}
	for _, spec := range moeSpecs {
		prefill, decode, err := PlanPD(PDPlanInput{
			Model: spec,
			GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
		})
		require.NoError(t, err)
		require.Equal(t, prefill.TP, prefill.EP, "prefill EP must equal TP for MoE model %v", spec)
		require.Equal(t, decode.TP, decode.EP, "decode EP must equal TP for MoE model %v", spec)
	}

	// Dense model: EP should always be 1
	denseSpec := PDModelSpec{TotalParamsB: 72, NonMoEParamsB: 72, TotalExperts: 0, Precision: "bf16"}
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: denseSpec,
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 1, prefill.EP, "prefill EP must be 1 for dense model")
	require.Equal(t, 1, decode.EP, "decode EP must be 1 for dense model")
}

func TestPlanPD_TotalGPUsEqualsTP(t *testing.T) {
	// TotalGPUs must equal TP (not TP*EP*DP), since DP=1 and EP follows TP.
	specs := []PDModelSpec{
		{TotalParamsB: 235, NonMoEParamsB: 18.92, TotalExperts: 128, Precision: "bf16"},
		{TotalParamsB: 72, NonMoEParamsB: 72, TotalExperts: 0, Precision: "bf16"},
		{TotalParamsB: 30, NonMoEParamsB: 3, TotalExperts: 128, Precision: "bf16"},
	}
	for _, spec := range specs {
		prefill, decode, err := PlanPD(PDPlanInput{
			Model: spec,
			GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
		})
		require.NoError(t, err)
		require.Equal(t, prefill.TP, prefill.TotalGPUs,
			"prefill TotalGPUs (%d) must equal TP (%d) for spec %v", prefill.TotalGPUs, prefill.TP, spec)
		require.Equal(t, decode.TP, decode.TotalGPUs,
			"decode TotalGPUs (%d) must equal TP (%d) for spec %v", decode.TotalGPUs, decode.TP, spec)
		require.Equal(t, 1, prefill.DP, "DP must be 1")
		require.Equal(t, 1, decode.DP, "DP must be 1")
	}
}

// ===== Real model tests from OpenCSG config.json =====

// TestPlanPDRecommendation_DeepSeekV4Flash_RealConfig tests PD planning with
// the actual config.json parameters from deepseek-ai/DeepSeek-V4-Flash on OpenCSG.
//
// config.json values:
//   hidden_size=4096, num_hidden_layers=43, n_routed_experts=256,
//   num_experts_per_tok=6, quantization_config.quant_method=fp8, torch_dtype=bfloat16
//
// NonMoE = 43 * 12 * 4096^2 / 1e9 = 8.657B
// Expert = 284 - 8.657 = 275.343B
// TP=8: (8.657/8 + 275.343/8) * 1.0 * 1.15 = 40.83GB < 68GB budget (80*0.85) — fits single node!
func TestPlanPDRecommendation_DeepSeekV4Flash_RealConfig(t *testing.T) {
	rec, err := PlanPDRecommendation(
		"deepseek-ai/DeepSeek-V4-Flash",
		284,    // totalParamsB
		256,    // totalExperts (n_routed_experts)
		6,      // activeExperts (num_experts_per_tok)
		"fp8",  // precision (quantization_config.quant_method)
		4096,   // hiddenSize
		43,     // numHiddenLayers
		220,    // minInferenceVRAMGB (from metadata.mini_gpu_memory_gb)
	)
	require.NoError(t, err)
	require.NotNil(t, rec)

	// NonMoE estimated from hidden_size and num_hidden_layers
	require.InDelta(t, 8.657, rec.NonMoEParamsB, 0.01)

	// TP=8 fits on single node (40.83GB < 68GB budget)
	require.Equal(t, 8, rec.Prefill.TP)
	require.Equal(t, 8, rec.Prefill.EP)
	require.Equal(t, 8, rec.Prefill.TotalGPUs)
	require.Equal(t, 1, rec.Prefill.Pods) // single node

	require.Equal(t, 8, rec.Decode.TP)
	require.Equal(t, 8, rec.Decode.TotalGPUs)
	require.Equal(t, 1, rec.Decode.Pods)

	// TotalVRAMGB = MinInferenceVRAMGB * Pods = 220 * 1 = 220
	require.Equal(t, 220.0, rec.Prefill.TotalVRAMGB)
	require.Equal(t, 220.0, rec.Decode.TotalVRAMGB)

	t.Logf("DeepSeek-V4-Flash: NonMoE=%.3fB Expert=%.3fB", rec.NonMoEParamsB, rec.TotalParamsB-rec.NonMoEParamsB)
	t.Logf("DeepSeek-V4-Flash: Prefill TP=%d EP=%d GPUs=%d Pods=%d VRAM=%.0f | Decode TP=%d EP=%d GPUs=%d Pods=%d VRAM=%.0f",
		rec.Prefill.TP, rec.Prefill.EP, rec.Prefill.TotalGPUs, rec.Prefill.Pods, rec.Prefill.TotalVRAMGB,
		rec.Decode.TP, rec.Decode.EP, rec.Decode.TotalGPUs, rec.Decode.Pods, rec.Decode.TotalVRAMGB)
}

// TestPlanPDRecommendation_KimiK3_RealConfig tests PD planning with the actual
// config.json parameters from moonshotai/Kimi-K3 on OpenCSG.
//
// config.json values (text_config):
//   hidden_size=7168, num_hidden_layers=93, num_experts=896,
//   num_experts_per_tok=16, quantization_config: mxfp4 (4-bit float),
//   dtype=bfloat16
//
// The model is ~2.8T parameters. mxfp4 quantization is not yet supported by the
// planner (no "mxfp4" in precisionBytes), so it falls back to fp8 (1.0 bytes/param),
// which is a conservative overestimate.
//
// NonMoE = 93 * 12 * 7168^2 / 1e9 = 57.36B
// Expert = 2800 - 57.36 = 2742.64B
//
// On 80GB GPU (PlanPDRecommendation default):
//   TP=32: 100.6GB > 68GB budget — infeasible
//   TP=64: 896 % 64 = 0, (57.36/64 + 2742.64/64) * 1.15 = 50.3GB < 68GB — fits! (8 nodes)
//
// On H200 (141GB) via PlanPD directly:
//   TP=32: 100.6GB < 119.85GB budget — fits on 4 nodes!
//   896 % 32 = 0, so EP=TP=32 is valid.
func TestPlanPDRecommendation_KimiK3_RealConfig(t *testing.T) {
	// PlanPDRecommendation uses 80GB GPU by default.
	// K3 fits on 80GB with TP=64 (8 nodes of 8 GPUs).
	rec, err := PlanPDRecommendation(
		"moonshotai/Kimi-K3",
		2800,   // totalParamsB (~2.8T)
		896,    // totalExperts (num_experts)
		16,     // activeExperts (num_experts_per_tok)
		"fp8",  // precision (mxfp4 not supported, falls back to fp8 conservative)
		7168,   // hiddenSize
		93,     // numHiddenLayers
		1840,   // minInferenceVRAMGB
	)
	require.NoError(t, err)
	require.NotNil(t, rec)

	// On 80GB, TP=64 is the minimum that fits: 50.3GB < 68GB budget.
	require.Equal(t, 64, rec.Prefill.TP)
	require.Equal(t, 64, rec.Prefill.EP)
	require.Equal(t, 64, rec.Prefill.TotalGPUs)
	require.Equal(t, 8, rec.Prefill.Pods) // 8 nodes of 8 GPUs

	require.Equal(t, 64, rec.Decode.TP)
	require.Equal(t, 64, rec.Decode.TotalGPUs)
	require.Equal(t, 8, rec.Decode.Pods)

	// TotalVRAMGB = MinInferenceVRAMGB * Pods = 1840 * 8 = 14720
	require.Equal(t, 1840.0*8, rec.Prefill.TotalVRAMGB)

	t.Logf("Kimi-K3 80GB: Prefill TP=%d EP=%d GPUs=%d Pods=%d VRAM=%.0f",
		rec.Prefill.TP, rec.Prefill.EP, rec.Prefill.TotalGPUs, rec.Prefill.Pods, rec.Prefill.TotalVRAMGB)
	t.Logf("Kimi-K3 80GB: Decode TP=%d EP=%d GPUs=%d Pods=%d VRAM=%.0f",
		rec.Decode.TP, rec.Decode.EP, rec.Decode.TotalGPUs, rec.Decode.Pods, rec.Decode.TotalVRAMGB)

	// On H200 (141GB), K3 fits with multi-node TP=32 (4 nodes of 8 GPUs).
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{
			TotalParamsB:  2800,
			NonMoEParamsB: 57.36,
			TotalExperts:  896,
			Precision:     "fp8",
		},
		GPU: PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Equal(t, 32, prefill.TP)
	require.Equal(t, 32, prefill.EP)
	require.Equal(t, 32, prefill.TotalGPUs)
	require.Equal(t, 4, prefill.LWSSize) // 4 nodes of 8 GPUs
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)

	require.Equal(t, 32, decode.TP)
	require.Equal(t, 32, decode.TotalGPUs)
	require.Equal(t, 4, decode.LWSSize)

	// Verify weight memory: (57.36/32 + 2742.64/32) * 1.0 * 1.15 = 100.6GB
	require.InDelta(t, 100.6, prefill.WeightMemPerGPU, 0.5)

	t.Logf("Kimi-K3 H200: Prefill TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB kv=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU, prefill.RemainingVRAMForKV)
	t.Logf("Kimi-K3 H200: Decode TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB kv=%.2fGB",
		decode.TP, decode.EP, decode.TotalGPUs, decode.LWSSize, decode.WeightMemPerGPU, decode.RemainingVRAMForKV)
}
