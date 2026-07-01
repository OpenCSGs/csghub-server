// pd_recommendation_ee.go — PD Recommendation (recommendation data structures and hardware matching)
//
// Responsibility of Recommendation:
//   Defines the data structures for PD recommendation results (PDRecommendation, PDRoleConfig),
//   and methods for matching/splitting recommendation results against actual cluster hardware.
//   Recommendation results are stored as JSONB in the metadata table.
//
// Core types:
//   - PDRecommendation: full PD recommendation (prefill + decode configs)
//   - PDRoleConfig: parallelism and resource config for a single role (prefill/decode)
//   - ModelArchType: model architecture classification (dense/moe/hybrid)
//
// Difference from pd_planner_ee.go:
//   - Planner (pd_planner_ee.go): responsible for "computation" — given model specs and GPU specs,
//     outputs the optimal parallelism strategy
//   - Recommendation (this file): responsible for "description and matching" — defines the data
//     structures for recommendation results and methods to match them against hardware resources

package types

// ModelArchType describes the architecture classification of a model.
// This is stored in the metadata table to help manage and query model information.
type ModelArchType string

const (
	// ModelArchTypeDense is a standard dense (non-MoE) model.
	ModelArchTypeDense ModelArchType = "dense"
	// ModelArchTypeMoE is a Mixture-of-Experts model with routed experts.
	ModelArchTypeMoE ModelArchType = "moe"
	// ModelArchTypeHybrid is a model that combines dense and MoE layers
	// (e.g., some layers use MoE while others are dense).
	ModelArchTypeHybrid ModelArchType = "hybrid"
)

// String returns the string representation of ModelArchType.
func (m ModelArchType) String() string {
	return string(m)
}

// IsMoE returns true if the architecture type is MoE or hybrid (has expert layers).
func (m ModelArchType) IsMoE() bool {
	return m == ModelArchTypeMoE || m == ModelArchTypeHybrid
}

// DetectModelArchType determines the model architecture type from expert counts.
// If totalExperts > 0, it's MoE; otherwise it's dense.
func DetectModelArchType(totalExperts int) ModelArchType {
	if totalExperts > 0 {
		return ModelArchTypeMoE
	}
	return ModelArchTypeDense
}

// PDRoleConfig describes the recommended parallelism and resource configuration
// for one PD role (prefill or decode). It is designed to be easily comparable
// with types.HardWare / types.Processor so that a hardware resource can be
// matched against the recommendation.
type PDRoleConfig struct {
	// TP is the tensor parallelism degree.
	TP int `json:"tp"`
	// EP is the expert parallelism degree (1 for dense models).
	EP int `json:"ep"`
	// DP is the data parallelism degree (number of model replicas).
	// DP replicates the model across groups for higher throughput.
	// TotalGPUs = TP * EP * DP.
	DP int `json:"dp"`
	// TotalGPUs is the total number of GPUs required (TP * EP * DP).
	TotalGPUs int `json:"total_gpus"`
	// Pods is the number of pods per LWS group (maps to LWS spec.leaderWorkerTemplate.size).
	// Each pod runs one vLLM/SGLang instance.
	// Example: TotalGPUs=8, GPUsPerPod=4 → Pods=2 → LWS Size=2 → 2 pods × 4 GPUs each.
	// When Pods=1 and TotalGPUs=4, all 4 GPUs are in a single pod;
	// HardWare.Replicas is set to Pods (1), and Gpu.Num is set to TotalGPUs/Pods (4).
	// The LWS Replicas field (number of LWS groups) is controlled separately by
	// PDConfig.PrefillReplicas/DecodeReplicas (default 1), which HPA scales up/down.
	Pods int `json:"pods"`
	// TotalVRAMGB is the total VRAM required for this role, computed as
	// MinInferenceVRAMGB * TotalGPUs. Used for VRAM validation and hardware splitting ratio.
	TotalVRAMGB float64 `json:"total_vram_gb"`
}

// PDRecommendation holds the full PD disaggregation recommendation for a model.
// It is stored as a JSONB column on the metadata table and is only populated
// once for models with >128B parameters and MoE architecture. Manual adjustments
// after the initial population are preserved (the field is not overwritten).
type PDRecommendation struct {
	// ModelName is the model name used to resolve the spec.
	ModelName string `json:"model_name,omitempty"`
	// TotalParamsB is the total parameter count in billions.
	TotalParamsB float64 `json:"total_params_b"`
	// NonMoEParamsB is the non-expert parameter count in billions.
	NonMoEParamsB float64 `json:"non_moe_params_b"`
	// TotalExperts is the number of routed experts (0 for dense models).
	TotalExperts int `json:"total_experts"`
	// ActiveExperts is the number of experts activated per token.
	ActiveExperts int `json:"active_experts"`
	// Precision is the inference precision.
	Precision string `json:"precision"`
	// MinInferenceVRAMGB is the minimum VRAM per GPU required to load and run inference.
	// TotalVRAMGB for each role is computed as MinInferenceVRAMGB * TotalGPUs.
	MinInferenceVRAMGB float64 `json:"min_inference_vram_gb"`
	// Prefill is the recommended prefill configuration.
	Prefill PDRoleConfig `json:"prefill"`
	// Decode is the recommended decode configuration.
	Decode PDRoleConfig `json:"decode"`
}

// IsEmpty returns true if the recommendation has not been populated.
func (r *PDRecommendation) IsEmpty() bool {
	if r == nil {
		return true
	}
	return r.TotalGPUs() == 0
}

// TotalGPUs returns the total GPUs across prefill and decode.
func (r *PDRecommendation) TotalGPUs() int {
	if r == nil {
		return 0
	}
	return r.Prefill.TotalGPUs + r.Decode.TotalGPUs
}
