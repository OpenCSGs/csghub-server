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
	require.GreaterOrEqual(t, prefill.TP, 4)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.LessOrEqual(t, decode.TP, 4)

	t.Logf("DeepSeek-V3 H200: Prefill TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB kv=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU, prefill.RemainingVRAMForKV)
	t.Logf("DeepSeek-V3 H200: Decode TP=%d EP=%d GPUs=%d LWSSize=%d weight=%.2fGB kv=%.2fGB",
		decode.TP, decode.EP, decode.TotalGPUs, decode.LWSSize, decode.WeightMemPerGPU, decode.RemainingVRAMForKV)
}

func TestPlanPD_DeepSeekV3_A800(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 671, NonMoEParamsB: 37, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 80, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("DeepSeek-V3 A800: Prefill TP=%d EP=%d GPUs=%d weight=%.2fGB kv=%.2fGB",
		prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.WeightMemPerGPU, prefill.RemainingVRAMForKV)
	t.Logf("DeepSeek-V3 A800: Decode TP=%d EP=%d GPUs=%d weight=%.2fGB kv=%.2fGB",
		decode.TP, decode.EP, decode.TotalGPUs, decode.WeightMemPerGPU, decode.RemainingVRAMForKV)
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
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 1600, NonMoEParamsB: 49, TotalExperts: 512, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("DeepSeek-V4-Pro H200: Prefill TP=%d EP=%d GPUs=%d", prefill.TP, prefill.EP, prefill.TotalGPUs)
	t.Logf("DeepSeek-V4-Pro H200: Decode TP=%d EP=%d GPUs=%d", decode.TP, decode.EP, decode.TotalGPUs)
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

func TestPlanPD_GLM52_H200(t *testing.T) {
	prefill, decode, err := PlanPD(PDPlanInput{
		Model: PDModelSpec{TotalParamsB: 1000, NonMoEParamsB: 50, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	require.Greater(t, prefill.RemainingVRAMForKV, 0.0)
	require.Greater(t, decode.RemainingVRAMForKV, 0.0)

	t.Logf("GLM-5.2 H200: Prefill TP=%d EP=%d GPUs=%d", prefill.TP, prefill.EP, prefill.TotalGPUs)
	t.Logf("GLM-5.2 H200: Decode TP=%d EP=%d GPUs=%d", decode.TP, decode.EP, decode.TotalGPUs)
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
	weight := calcWeightMemPerGPU(72, 0, 2, 1, 2.0)
	require.InDelta(t, 82.8, weight, 0.01)
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
		Model: PDModelSpec{TotalParamsB: 671, NonMoEParamsB: 37, TotalExperts: 256, Precision: "fp8"},
		GPU:   PDGPUConfig{VRAMGB: 141, GPUsPerNode: 8},
	})
	require.NoError(t, err)
	if decode.TotalGPUs > 8 {
		expectedSize := int(math.Ceil(float64(decode.TotalGPUs) / 8.0))
		require.Equal(t, expectedSize, decode.LWSSize)
		require.Equal(t, 8, decode.GPUsPerPod)
	}
}

func TestBuildEPCandidates_Dense(t *testing.T) {
	candidates := buildEPCandidates(0, false)
	require.Equal(t, []int{1}, candidates)
}

func TestBuildEPCandidates_MoE(t *testing.T) {
	candidates := buildEPCandidates(256, true)
	require.Contains(t, candidates, 1)
	require.Contains(t, candidates, 256)
	require.Contains(t, candidates, 8)
	for _, ep := range candidates {
		require.Equal(t, 0, 256%ep, "EP %d should be a factor of 256", ep)
	}
}

func TestPlanPD_FullMatrix(t *testing.T) {
	models := map[string]PDModelSpec{
		"DeepSeek-V3":       {TotalParamsB: 671, NonMoEParamsB: 37, TotalExperts: 256, Precision: "fp8"},
		"DeepSeek-V4-Flash": {TotalParamsB: 284, NonMoEParamsB: 13, TotalExperts: 256, Precision: "fp8"},
		"DeepSeek-V4-Pro":   {TotalParamsB: 1600, NonMoEParamsB: 49, TotalExperts: 512, Precision: "fp8"},
		"GLM-5.1":           {TotalParamsB: 744, NonMoEParamsB: 40, TotalExperts: 256, Precision: "fp8"},
		"GLM-5.2":           {TotalParamsB: 1000, NonMoEParamsB: 50, TotalExperts: 256, Precision: "fp8"},
		"GPT-120B":          {TotalParamsB: 120, NonMoEParamsB: 120, TotalExperts: 0, Precision: "bf16"},
		"Qwen3-30B-A3B":     {TotalParamsB: 30, NonMoEParamsB: 3, TotalExperts: 128, Precision: "bf16"},
		"Qwen1.5-MoE":       {TotalParamsB: 14.3, NonMoEParamsB: 2.5, TotalExperts: 60, Precision: "bf16"},
	}

	// infeasible combinations: model too large for GPU with 32-GPU limit
	infeasible := map[string]bool{
		"DeepSeek-V4-Pro_A800": true, // 1600B model needs >32 A800 GPUs
	}

	gpus := map[string]PDGPUConfig{
		"A800": {VRAMGB: 80, GPUsPerNode: 8},
		"H200": {VRAMGB: 141, GPUsPerNode: 8},
	}

	for modelName, spec := range models {
		for gpuName, gpu := range gpus {
			key := modelName + "_" + gpuName
			t.Run(key, func(t *testing.T) {
				if infeasible[key] {
					t.Skipf("skipping infeasible combination: %s on %s", modelName, gpuName)
				}
				prefill, decode, err := PlanPD(PDPlanInput{Model: spec, GPU: gpu})
				require.NoError(t, err, "PlanPD should succeed for %s on %s", modelName, gpuName)
				require.Greater(t, prefill.RemainingVRAMForKV, 0.0, "prefill should have KV headroom")
				require.Greater(t, decode.RemainingVRAMForKV, 0.0, "decode should have KV headroom")

				t.Logf("%-20s %-5s | Prefill: TP=%2d EP=%3d GPUs=%2d Size=%d weight=%.1fGB | Decode: TP=%2d EP=%3d GPUs=%2d Size=%d weight=%.1fGB",
					modelName, gpuName,
					prefill.TP, prefill.EP, prefill.TotalGPUs, prefill.LWSSize, prefill.WeightMemPerGPU,
					decode.TP, decode.EP, decode.TotalGPUs, decode.LWSSize, decode.WeightMemPerGPU)
			})
		}
	}
}

func TestPlanPDRecommendation_DeepSeekV3(t *testing.T) {
	rec, err := PlanPDRecommendation("deepseek-v3", 671, 256, 8, "fp8", 7168, 61)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 671.0, rec.TotalParamsB)
	require.Equal(t, 256, rec.TotalExperts)
	require.Equal(t, 8, rec.ActiveExperts)
	require.Equal(t, "fp8", rec.Precision)
	require.Equal(t, 80.0, rec.MinInferenceVRAMGB)
	require.Greater(t, rec.Prefill.TotalGPUs, 0)
	require.Greater(t, rec.Decode.TotalGPUs, 0)
	require.Greater(t, rec.Prefill.TotalVRAMGB, 0.0)
	require.Greater(t, rec.Prefill.Pods, 0)
	require.Greater(t, rec.Decode.Pods, 0)

	t.Logf("DeepSeek-V3 80G: Prefill TP=%d EP=%d GPUs=%d Pods=%d | Decode TP=%d EP=%d GPUs=%d Pods=%d",
		rec.Prefill.TP, rec.Prefill.EP, rec.Prefill.TotalGPUs, rec.Prefill.Pods,
		rec.Decode.TP, rec.Decode.EP, rec.Decode.TotalGPUs, rec.Decode.Pods)
}

func TestPlanPDRecommendation_GLM52(t *testing.T) {
	rec, err := PlanPDRecommendation("glm-5.2", 1000, 256, 8, "fp8", 8192, 80)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 1000.0, rec.TotalParamsB)
	require.Greater(t, rec.Prefill.TotalGPUs, 0)
	require.Greater(t, rec.Decode.TotalGPUs, 0)
}

func TestPlanPDRecommendation_Qwen3_30B_A3B(t *testing.T) {
	rec, err := PlanPDRecommendation("qwen3-30b-a3b", 30, 128, 8, "bf16", 2048, 48)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 30.0, rec.TotalParamsB)
	require.Equal(t, 128, rec.TotalExperts)
}

func TestPlanPDRecommendation_UnknownModelMoE(t *testing.T) {
	// Unknown MoE model with expert info from config.json
	rec, err := PlanPDRecommendation("custom/moe-model", 200, 64, 8, "bf16", 4096, 32)
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
	rec, err := PlanPDRecommendation("custom-dense-70b", 70, 0, 0, "bf16", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, 70.0, rec.TotalParamsB)
	require.Equal(t, 0, rec.TotalExperts)
	require.Equal(t, 70.0, rec.NonMoEParamsB)
}

func TestPlanPDRecommendation_ZeroParams(t *testing.T) {
	_, err := PlanPDRecommendation("test", 0, 256, 8, "fp8", 0, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "total params must be positive")
}

func TestPlanPDRecommendation_NegativeParams(t *testing.T) {
	_, err := PlanPDRecommendation("test", -10, 256, 8, "fp8", 0, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "total params must be positive")
}

func TestPlanPDRecommendation_PrecisionOverride(t *testing.T) {
	// Known model with explicit precision override
	// Use a smaller model that fits in 80GB with bf16
	rec, err := PlanPDRecommendation("qwen3-30b-a3b", 30, 128, 8, "bf16", 2048, 48)
	require.NoError(t, err)
	require.Equal(t, "bf16", rec.Precision)
}

func TestPlanPDRecommendation_ExpertOverride(t *testing.T) {
	// Override expert count from config.json
	rec, err := PlanPDRecommendation("deepseek-v3", 671, 512, 16, "fp8", 7168, 61)
	require.NoError(t, err)
	require.Equal(t, 512, rec.TotalExperts)
	require.Equal(t, 16, rec.ActiveExperts)
}

func TestPlanPDRecommendation_DPField(t *testing.T) {
	rec, err := PlanPDRecommendation("deepseek-v3", 671, 256, 8, "fp8", 7168, 61)
	require.NoError(t, err)
	// DP should default to 1 for all recommendations
	require.Equal(t, 1, rec.Prefill.DP)
	require.Equal(t, 1, rec.Decode.DP)
	// TotalGPUs should equal TP * EP * DP
	require.Equal(t, rec.Prefill.TP*rec.Prefill.EP*rec.Prefill.DP, rec.Prefill.TotalGPUs)
	require.Equal(t, rec.Decode.TP*rec.Decode.EP*rec.Decode.DP, rec.Decode.TotalGPUs)
}
