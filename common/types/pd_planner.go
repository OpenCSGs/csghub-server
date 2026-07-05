// pd_planner_ee.go — PD Planner (parallelism strategy planner)
//
// Responsibility of Planner:
//   Given model parameter count, expert count, precision and other characteristics,
//   automatically computes the optimal PD disaggregated deployment parallelism config
//   (TP/EP/DP), and estimates per-GPU weight memory usage and remaining KV cache space.
//
// Core functions:
//   - PlanPD: enumerates all valid (TP, EP, DP) combinations and selects the optimal config
//     using different heuristics for prefill vs decode
//   - PlanPDRecommendation: estimates NonMoE parameter count from config.json metadata
//     (hidden_size, num_hidden_layers, etc.), calls PlanPD to generate a PDRecommendation,
//     and stores it in the metadata table
//
// Difference from pd_recommendation_ee.go:
//   - Planner (this file): responsible for "computation" — given model specs and GPU specs,
//     outputs the optimal parallelism strategy
//   - Recommendation (pd_recommendation_ee.go): responsible for "description and matching" —
//     defines the data structures for recommendation results and methods to match them
//     against hardware resources

package types

import (
	"fmt"
	"math"
	"strings"
)

// ===== Precision Constants =====

// precisionBytes maps precision strings to bytes per parameter.
var precisionBytes = map[string]float64{
	"fp16": 2.0,
	"bf16": 2.0,
	"fp8":  1.0,
	"int8": 1.0,
	"int4": 0.5,
}

// systemOverheadFactor accounts for CUDA context, activations, and framework workspace.
const systemOverheadFactor = 1.15

// defaultKVRatioPrefill is the fraction of GPU VRAM reserved for KV cache in prefill.
const defaultKVRatioPrefill = 0.15

// defaultKVRatioDecode is the fraction of GPU VRAM reserved for KV cache in decode.
const defaultKVRatioDecode = 0.35

// ===== PD Planning Algorithm =====

// pdCandidate represents a valid (TP, EP, DP) configuration.
type pdCandidate struct {
	TP        int
	EP        int
	DP        int
	WeightMem float64
	TotalGPUs int
}

// PlanPD computes the optimal prefill and decode LWS configurations for PD disaggregation.
//
// Algorithm overview:
//  1. Enumerate all (TP, EP) combinations satisfying divisibility constraints.
//  2. For each candidate, estimate per-GPU weight memory using:
//     M = (NonMoE/TP + Expert/(TP*EP)) * B * alpha
//  3. Select the best prefill config: maximize TP (within single node), minimize total GPUs.
//  4. Select the best decode config: minimize TP (<=2 preferred), maximize EP, minimize total GPUs.
//
// Returns (prefillResult, decodeResult, error). Returns an error if no valid config is found.
func PlanPD(input PDPlanInput) (prefill PDPlanResult, decode PDPlanResult, err error) {
	if input.KVRatioPrefill <= 0 {
		input.KVRatioPrefill = defaultKVRatioPrefill
	}
	if input.KVRatioDecode <= 0 {
		input.KVRatioDecode = defaultKVRatioDecode
	}

	spec := input.Model
	bParam, ok := precisionBytes[strings.ToLower(spec.Precision)]
	if !ok {
		bParam = 1.0 // Default to fp8 if unknown
	}

	isMoE := spec.TotalExperts > 0
	expertParamsB := spec.TotalParamsB - spec.NonMoEParamsB

	// Build TP candidates: powers of 2, limited to GPUsPerNode for NVLink efficiency
	tpCandidates := buildTPCandidates(input.GPU.GPUsPerNode)

	// Build EP candidates: for MoE models, factors of TotalExperts; for dense, only EP=1
	epCandidates := buildEPCandidates(spec.TotalExperts, isMoE)

	// Build DP candidates: powers of 2, default [1].
	// DP replicates the model for higher throughput; it does not reduce per-GPU memory.
	dpCandidates := []int{1}

	// Enumerate all valid candidates
	candidates := make([]pdCandidate, 0, len(tpCandidates)*len(epCandidates)*len(dpCandidates))
	for _, tp := range tpCandidates {
		for _, ep := range epCandidates {
			weightMem := calcWeightMemPerGPU(spec.NonMoEParamsB, expertParamsB, tp, ep, bParam)
			for _, dp := range dpCandidates {
				candidates = append(candidates, pdCandidate{
					TP:        tp,
					EP:        ep,
					DP:        dp,
					WeightMem: weightMem,
					TotalGPUs: tp * ep * dp,
				})
			}
		}
	}

	// Apply MaxTotalGPUs limit (default: 32 = 4 nodes of 8 GPUs)
	maxGPUs := input.MaxTotalGPUs
	if maxGPUs <= 0 {
		maxGPUs = 32
	}

	// Filter candidates by MaxTotalGPUs
	filtered := make([]pdCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.TotalGPUs <= maxGPUs {
			filtered = append(filtered, c)
		}
	}

	// Plan prefill: maximize TP, prefer single node, minimize total GPUs
	prefill, err = planPrefill(filtered, input.GPU.VRAMGB, input.KVRatioPrefill, input.GPU.GPUsPerNode)
	if err != nil {
		return PDPlanResult{}, PDPlanResult{}, fmt.Errorf("prefill planning failed: %w", err)
	}

	// Plan decode: minimize TP (<=2 preferred), maximize EP, minimize total GPUs
	decode, err = planDecode(filtered, input.GPU.VRAMGB, input.KVRatioDecode, input.GPU.GPUsPerNode)
	if err != nil {
		return PDPlanResult{}, PDPlanResult{}, fmt.Errorf("decode planning failed: %w", err)
	}

	return prefill, decode, nil
}

// calcWeightMemPerGPU estimates the model weight memory per GPU in GB.
// Formula: (NonMoE/TP + Expert/(TP*EP)) * B * alpha
//
// Preconditions: tp >= 1 and ep >= 1 (callers use buildTPCandidates/buildEPCandidates
// which always return values >= 1). Violating this would cause division by zero.
func calcWeightMemPerGPU(nonMoEParamsB, expertParamsB float64, tp, ep int, bParam float64) float64 {
	return (nonMoEParamsB/float64(tp) + expertParamsB/(float64(tp)*float64(ep))) * bParam * systemOverheadFactor
}

// buildTPCandidates returns TP values as powers of 2, not exceeding GPUsPerNode.
func buildTPCandidates(gpusPerNode int) []int {
	powers := []int{1, 2, 4, 8, 16, 32, 64}
	result := make([]int, 0, len(powers))
	for _, tp := range powers {
		if tp <= gpusPerNode {
			result = append(result, tp)
		}
	}
	return result
}

// buildEPCandidates returns EP values for expert parallelism.
// For dense models (totalExperts==0), returns [1].
// For MoE models, returns all factors of totalExperts.
func buildEPCandidates(totalExperts int, isMoE bool) []int {
	if !isMoE || totalExperts <= 0 {
		return []int{1}
	}
	factors := make([]int, 0)
	for i := 1; i <= totalExperts; i++ {
		if totalExperts%i == 0 {
			factors = append(factors, i)
		}
	}
	return factors
}

// planPrefill selects the best prefill configuration.
// Heuristic: maximize TP (for compute throughput), prefer single node (NVLink),
// but allow multi-node with EP if the model is too large for a single node.
func planPrefill(candidates []pdCandidate, gpuVRAMGB, kvRatio float64, gpusPerNode int) (PDPlanResult, error) {
	maxWeight := gpuVRAMGB * (1 - kvRatio)

	// Primary: single-node candidates (TP*EP <= gpusPerNode)
	var singleNode []pdCandidate
	for _, c := range candidates {
		if c.WeightMem <= maxWeight && c.TotalGPUs <= gpusPerNode {
			singleNode = append(singleNode, c)
		}
	}

	if len(singleNode) > 0 {
		// Sort: minimize total GPUs first, then maximize TP, then minimize EP
		sortCandidates(singleNode, func(a, b pdCandidate) bool {
			if a.TotalGPUs != b.TotalGPUs {
				return a.TotalGPUs < b.TotalGPUs
			}
			if a.TP != b.TP {
				return a.TP > b.TP
			}
			return a.EP < b.EP
		})
		return buildPlanResult(singleNode[0], gpuVRAMGB, gpusPerNode), nil
	}

	// Fallback: allow multi-node if single-node is not feasible
	var multiNode []pdCandidate
	for _, c := range candidates {
		if c.WeightMem <= maxWeight {
			multiNode = append(multiNode, c)
		}
	}

	if len(multiNode) == 0 {
		return PDPlanResult{}, fmt.Errorf("no valid prefill config: model cannot fit with %.0fGB VRAM", gpuVRAMGB)
	}

	// Sort: minimize total GPUs, then maximize TP
	sortCandidates(multiNode, func(a, b pdCandidate) bool {
		if a.TotalGPUs != b.TotalGPUs {
			return a.TotalGPUs < b.TotalGPUs
		}
		return a.TP > b.TP
	})

	return buildPlanResult(multiNode[0], gpuVRAMGB, gpusPerNode), nil
}

// planDecode selects the best decode configuration.
// Heuristic: minimize TP (TP<=2 preferred for small-batch efficiency),
// maximize EP (spread expert weights across more GPUs for KV cache headroom),
// minimize total GPUs.
func planDecode(candidates []pdCandidate, gpuVRAMGB, kvRatio float64, gpusPerNode int) (PDPlanResult, error) {
	maxWeight := gpuVRAMGB * (1 - kvRatio)

	// Primary search: TP <= 2
	var primary []pdCandidate
	for _, c := range candidates {
		if c.WeightMem <= maxWeight && c.TP <= 2 {
			primary = append(primary, c)
		}
	}

	var best pdCandidate
	found := false

	if len(primary) > 0 {
		// Sort: minimize TP, then minimize total GPUs, then maximize EP
		sortCandidates(primary, func(a, b pdCandidate) bool {
			if a.TP != b.TP {
				return a.TP < b.TP
			}
			if a.TotalGPUs != b.TotalGPUs {
				return a.TotalGPUs < b.TotalGPUs
			}
			return a.EP > b.EP
		})
		best = primary[0]
		found = true
	}

	// Fallback: relax TP constraint, search all valid configs
	if !found {
		var backup []pdCandidate
		for _, c := range candidates {
			if c.WeightMem <= maxWeight {
				backup = append(backup, c)
			}
		}
		if len(backup) > 0 {
			sortCandidates(backup, func(a, b pdCandidate) bool {
				if a.TP != b.TP {
					return a.TP < b.TP
				}
				return a.TotalGPUs < b.TotalGPUs
			})
			best = backup[0]
			found = true
		}
	}

	if !found {
		return PDPlanResult{}, fmt.Errorf("no valid decode config: model cannot fit with %.0fGB VRAM", gpuVRAMGB)
	}

	return buildPlanResult(best, gpuVRAMGB, gpusPerNode), nil
}

// buildPlanResult converts a candidate into a PDPlanResult with LWS layout.
func buildPlanResult(c pdCandidate, gpuVRAMGB float64, gpusPerNode int) PDPlanResult {
	var lwsSize int
	var gpusPerPod int
	totalGPUs := c.TotalGPUs

	if totalGPUs >= gpusPerNode {
		lwsSize = int(math.Ceil(float64(totalGPUs) / float64(gpusPerNode)))
		gpusPerPod = gpusPerNode
	} else {
		lwsSize = 1
		gpusPerPod = totalGPUs
	}

	remaining := gpuVRAMGB - c.WeightMem

	return PDPlanResult{
		TP:                 c.TP,
		EP:                 c.EP,
		DP:                 c.DP,
		TotalGPUs:          totalGPUs,
		LWSSize:            lwsSize,
		GPUsPerPod:         gpusPerPod,
		WeightMemPerGPU:    c.WeightMem,
		RemainingVRAMForKV: remaining,
	}
}

// sortCandidates sorts a slice of pdCandidate using a custom less function (insertion sort for small slices).
func sortCandidates(candidates []pdCandidate, less func(a, b pdCandidate) bool) {
	// Use insertion sort since candidate lists are typically small
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && less(candidates[j], candidates[j-1]); j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
}

// ===== PDRecommendation Generation from ModelInfo =====

// DefaultGPUUnitGB is the standard GPU VRAM unit (80GB) used for PD recommendations.
const DefaultGPUUnitGB = 80.0

// DefaultGPUsPerNode is the standard number of GPUs per node.
const DefaultGPUsPerNode = 8

// PlanPDRecommendation generates a PDRecommendation from model metadata parsed
// from config.json. It uses the 80GB GPU unit as the standard planning baseline.
//
// Parameters:
//   - modelName: repository path (used for logging only, no longer looked up in a registry)
//   - totalParamsB: total parameter count in billions (from config.json / safetensors)
//   - totalExperts: number of routed experts (0 for dense models)
//   - activeExperts: number of experts activated per token
//   - precision: inference precision (fp8, bf16, etc.)
//   - hiddenSize: hidden size from config.json (used to estimate NonMoEParamsB)
//   - numHiddenLayers: number of hidden layers from config.json
//
// The function:
//  1. Estimates NonMoEParamsB from hidden_size and num_hidden_layers when available,
//     otherwise falls back to the active/total expert ratio heuristic.
//  2. Plans prefill and decode configurations using PlanPD.
//  3. Returns a PDRecommendation suitable for storage as JSONB.
func PlanPDRecommendation(modelName string, totalParamsB float64, totalExperts, activeExperts int, precision string, hiddenSize, numHiddenLayers int) (*PDRecommendation, error) {
	if totalParamsB <= 0 {
		return nil, fmt.Errorf("total params must be positive, got %f", totalParamsB)
	}

	spec := PDModelSpec{
		TotalParamsB: totalParamsB,
		TotalExperts: totalExperts,
		Precision:    precision,
	}

	// Estimate NonMoEParamsB (non-expert parameters: attention + shared FFN + embedding).
	// When hidden_size and num_hidden_layers are available from config.json,
	// use the standard transformer parameter formula:
	//   NonMoE ≈ num_layers * 12 * hidden_size^2  (Q/K/V/O + shared FFN + layer norms)
	// This is more accurate than the active/total expert ratio heuristic.
	if hiddenSize > 0 && numHiddenLayers > 0 {
		spec.NonMoEParamsB = float64(numHiddenLayers) * 12.0 * float64(hiddenSize) * float64(hiddenSize) / 1e9
		// Cap at total params to avoid overestimation for small models
		if spec.NonMoEParamsB > spec.TotalParamsB {
			spec.NonMoEParamsB = spec.TotalParamsB
		}
	} else if spec.TotalExperts > 0 && activeExperts > 0 {
		// Fallback: estimate non-MoE params from active/total expert ratio
		spec.NonMoEParamsB = spec.TotalParamsB * float64(activeExperts) / float64(spec.TotalExperts)
	} else {
		// Dense model: all params are non-MoE
		spec.NonMoEParamsB = spec.TotalParamsB
	}

	// Plan with 80GB GPU unit
	gpuConfig := PDGPUConfig{
		VRAMGB:      DefaultGPUUnitGB,
		GPUsPerNode: DefaultGPUsPerNode,
	}

	prefill, decode, err := PlanPD(PDPlanInput{
		Model: spec,
		GPU:   gpuConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to plan PD for model %s: %w", modelName, err)
	}

	return &PDRecommendation{
		ModelName:          modelName,
		TotalParamsB:       spec.TotalParamsB,
		NonMoEParamsB:      spec.NonMoEParamsB,
		TotalExperts:       spec.TotalExperts,
		ActiveExperts:      activeExperts,
		Precision:          spec.Precision,
		MinInferenceVRAMGB: DefaultGPUUnitGB,
		Prefill:            planResultToRoleConfig(prefill),
		Decode:             planResultToRoleConfig(decode),
	}, nil
}

// planResultToRoleConfig converts a PDPlanResult into a PDRoleConfig.
// TotalVRAMGB is computed as MinInferenceVRAMGB * TotalGPUs by the caller
// (PlanPDRecommendation sets MinInferenceVRAMGB = DefaultGPUUnitGB).
func planResultToRoleConfig(result PDPlanResult) PDRoleConfig {
	pods := result.LWSSize
	if pods < 1 {
		pods = 1
	}
	return PDRoleConfig{
		TP:          result.TP,
		EP:          result.EP,
		DP:          result.DP,
		TotalGPUs:   result.TotalGPUs,
		Pods:        pods,
		TotalVRAMGB: float64(result.TotalGPUs) * DefaultGPUUnitGB,
	}
}
