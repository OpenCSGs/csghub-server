package types

const (
	// PD resource name suffixes
	PDPrefillSuffix = "-prefill"
	PDDecodeSuffix  = "-decode"
	PDEPPSuffix     = "-epp"

	// PD role values
	PDRolePrefill = "prefill"
	PDRoleDecode  = "decode"

	// PD engine types
	PDEngineVLLM   = "vllm"
	PDEngineSGLang = "sglang"

	// PD routing proxy connectors
	PDConnectorNixLV2 = "nixlv2"
	PDConnectorSGLang = "sglang"

	// PD label keys
	PDLabelGuide              = "llm-d.ai/guide"
	PDLabelModel              = "llm-d.ai/model"
	PDLabelEngineType         = "llm-d.ai/engine-type"
	PDLabelRole               = "llm-d.ai/role"
	PDLabelInferenceServing   = "llm-d.ai/inference-serving"
	PDLabelAcceleratorVendor  = "llm-d.ai/accelerator-vendor"
	PDLabelAcceleratorVariant = "llm-d.ai/accelerator-variant"

	// PD proxy resource overhead (CPU-only pods: knative nginx + EPP routing).
	// These are subtracted from the total hardware CPU/memory before splitting
	// between prefill and decode GPU roles.
	PDProxyCPURequest    = "1"   // 1 CPU request per proxy pod
	PDProxyCPULimit      = "2"   // 2 CPU limit per proxy pod
	PDProxyMemoryRequest = "1Gi" // 1Gi memory request per proxy pod
	PDProxyMemoryLimit   = "2Gi" // 2Gi memory limit per proxy pod
	// Number of CPU-only proxy pods: 1 knative nginx + 1 EPP routing
	PDProxyPodCount = 2

	// PD env var keys
	PDEnvRole      = "PD_ROLE"
	PDEnvGuide     = "PD_GUIDE"
	PDEnvEngine    = "PD_ENGINE"
	PDEnvModelName = "PD_MODEL_NAME"

	// PD model name env var keys (used for model name lookup)
	PDEnvRepoID       = "REPO_ID"
	PDEnvModelID      = "MODEL_ID"
	PDEnvModelNameEnv = "MODEL_NAME"

	// PD EPP env var keys
	PDEnvEPPPort     = "EPP_PORT"
	PDEnvPIPIndexURL = "PIP_INDEX_URL"

	// PD parallelism env var keys (exposed for standalone images and shell scripts)
	PDEnvTP = "PD_TP" // tensor parallelism degree
	PDEnvEP = "PD_EP" // expert parallelism degree
	PDEnvDP = "PD_DP" // data parallelism degree

	// PD HPA metric names
	PDMetricEPPQueueSize       = "epp_queue_size"
	PDMetricEPPRunningRequests = "epp_running_requests"

	// PD InferencePool constants
	PDInferencePoolAPIVersion  = "inference.networking.k8s.io/v1"
	PDInferencePoolKind        = "InferencePool"
	PDInferencePoolResource    = "inferencepools"
	PDInferencePoolFailureMode = "FailOpen"
	PDInferencePoolAppProtocol = "http"

	// PD HPA scale target constants
	PDHPAScaleTargetAPIVersion = "leaderworkerset.x-k8s.io/v1"
	PDHPAScaleTargetKind       = "LeaderWorkerSet"

	// PD container names
	PDLWSLeaderContainerName    = "leader"
	PDLWSWorkerContainerName    = "worker"
	PDRoutingProxyContainerName = "routing-proxy"

	// PD service port names
	PDEPPServicePortName       = "epp"
	PDEPPPublicServicePortName = "public"
	PDEPPMetricsPortName       = "http-metrics"

	// PD label value constants
	PDLabelAppKey = "app"
)

// PDAcceleratorVendor defines supported accelerator vendors for llm-d PD disaggregation.
// See: https://llm-d.ai/docs/accelerators
type PDAcceleratorVendor string

const (
	// PDAcceleratorVendorNVIDIA represents NVIDIA GPUs (default, CUDA-based)
	PDAcceleratorVendorNVIDIA PDAcceleratorVendor = "nvidia"
	// PDAcceleratorVendorAMD represents AMD GPUs (ROCm-based)
	PDAcceleratorVendorAMD PDAcceleratorVendor = "amd"
	// PDAcceleratorVendorGoogle represents Google TPUs
	PDAcceleratorVendorGoogle PDAcceleratorVendor = "google"
	// PDAcceleratorVendorIntel represents Intel XPU/HPU accelerators
	PDAcceleratorVendorIntel PDAcceleratorVendor = "intel"
	// PDAcceleratorVendorRebellions represents Rebellions NPUs
	PDAcceleratorVendorRebellions PDAcceleratorVendor = "rebellions"
	// PDAcceleratorVendorAscend represents Huawei Ascend NPUs
	PDAcceleratorVendorAscend PDAcceleratorVendor = "ascend"
	// PDAcceleratorVendorCPU represents CPU-only inference
	PDAcceleratorVendorCPU PDAcceleratorVendor = "cpu"
)

// PDAcceleratorVariant defines the accelerator hardware variant type.
type PDAcceleratorVariant string

const (
	// PDAcceleratorVariantGPU represents GPU accelerators (NVIDIA, AMD)
	PDAcceleratorVariantGPU PDAcceleratorVariant = "gpu"
	// PDAcceleratorVariantTPU represents Tensor Processing Units (Google)
	PDAcceleratorVariantTPU PDAcceleratorVariant = "tpu"
	// PDAcceleratorVariantNPU represents Neural Processing Units (Rebellions, Ascend)
	PDAcceleratorVariantNPU PDAcceleratorVariant = "npu"
	// PDAcceleratorVariantXPU represents Intel XPU accelerators
	PDAcceleratorVariantXPU PDAcceleratorVariant = "xpu"
	// PDAcceleratorVariantHPU represents Intel Gaudi HPU accelerators
	PDAcceleratorVariantHPU PDAcceleratorVariant = "hpu"
	// PDAcceleratorVariantCPU represents CPU-only inference
	PDAcceleratorVariantCPU PDAcceleratorVariant = "cpu"
)

// pdAcceleratorVendorVariant holds the llm-d accelerator vendor and variant for a hardware type.
type pdAcceleratorVendorVariant struct {
	Vendor  PDAcceleratorVendor
	Variant PDAcceleratorVariant
}

// hardwareTypeToPDAcceleratorVendorVariant maps hardware.Gpu.Type (and Npu/Gcu/etc. Type)
// values to the corresponding llm-d accelerator vendor and variant labels.
// See: https://llm-d.ai/docs/accelerators
var hardwareTypeToPDAcceleratorVendorVariant = map[string]pdAcceleratorVendorVariant{
	"nvidia":     {PDAcceleratorVendorNVIDIA, PDAcceleratorVariantGPU},
	"amd":        {PDAcceleratorVendorAMD, PDAcceleratorVariantGPU},
	"google":     {PDAcceleratorVendorGoogle, PDAcceleratorVariantTPU},
	"intel-xpu":  {PDAcceleratorVendorIntel, PDAcceleratorVariantXPU},
	"intel-hpu":  {PDAcceleratorVendorIntel, PDAcceleratorVariantHPU},
	"rebellions": {PDAcceleratorVendorRebellions, PDAcceleratorVariantNPU},
	"ascend":     {PDAcceleratorVendorAscend, PDAcceleratorVariantNPU},
	"cpu":        {PDAcceleratorVendorCPU, PDAcceleratorVariantCPU},
}

// PDAcceleratorFromHardware resolves llm-d accelerator vendor and variant labels from hardware config.
// It checks GPU, NPU, GCU, MLU, DCU, and GPGPU in order and returns the first match.
// If no accelerator is found, defaults to CPU vendor and variant.
func PDAcceleratorFromHardware(hw HardWare) (PDAcceleratorVendor, PDAcceleratorVariant) {
	// Check each accelerator type in priority order
	accelerators := []struct {
		typeStr string
		hasGPU  bool
	}{
		{hw.Gpu.Type, hw.Gpu.Num != "" && hw.Gpu.Num != "0"},
		{hw.Npu.Type, hw.Npu.Num != "" && hw.Npu.Num != "0"},
		{hw.Gcu.Type, hw.Gcu.Num != "" && hw.Gcu.Num != "0"},
		{hw.Mlu.Type, hw.Mlu.Num != "" && hw.Mlu.Num != "0"},
		{hw.Dcu.Type, hw.Dcu.Num != "" && hw.Dcu.Num != "0"},
		{hw.GPGpu.Type, hw.GPGpu.Num != "" && hw.GPGpu.Num != "0"},
	}

	for _, acc := range accelerators {
		if acc.hasGPU && acc.typeStr != "" {
			vv, ok := hardwareTypeToPDAcceleratorVendorVariant[acc.typeStr]
			if ok {
				return vv.Vendor, vv.Variant
			}
			// Unknown hardware type: use typeStr as vendor, default variant to gpu
			return PDAcceleratorVendor(acc.typeStr), PDAcceleratorVariantGPU
		}
	}

	// Default: CPU-only
	return PDAcceleratorVendorCPU, PDAcceleratorVariantCPU
}

// PDConfig holds configuration for PD (Prefill-Decode) disaggregation deployments.
type PDConfig struct {
	Enabled         bool `json:"enabled"`
	PrefillReplicas int  `json:"prefill_replicas,omitempty"`
	DecodeReplicas  int  `json:"decode_replicas,omitempty"`
	// Prefill holds the parallelism and hardware config for the prefill role.
	Prefill *PDRoleRuntimeConfig `json:"prefill,omitempty"`
	// Decode holds the parallelism and hardware config for the decode role.
	Decode *PDRoleRuntimeConfig `json:"decode,omitempty"`
	HPA    *PDHPAConfig         `json:"hpa,omitempty"`
}

// PDRoleRuntimeConfig holds the runtime parallelism and hardware resource
// configuration for one PD role (prefill or decode). This is provided by the
// client and validated/supplemented by the server.
type PDRoleRuntimeConfig struct {
	// TP is the tensor parallelism degree.
	TP int `json:"tp"`
	// EP is the expert parallelism degree (1 for dense models).
	EP int `json:"ep"`
	// DP is the data parallelism degree.
	DP int `json:"dp"`
	// TotalGPUs is the total GPUs for this role (TP * DP). EP does not add extra GPUs.
	TotalGPUs int `json:"total_gpus"`
	// PodsSize is the number of pods per LWS group (maps to LWS spec.leaderWorkerTemplate.size).
	// Each pod runs one vLLM/SGLang instance. GPUs per pod = TotalGPUs / PodsSize.
	// When PodsSize is 0 or 1, all GPUs are in a single pod.
	PodsSize int `json:"pods_size,omitempty"`
	// Hardware is the hardware resource allocated to this role.
	Hardware HardWare `json:"hardware"`
}

// PDHPAConfig holds HPA configuration for PD disaggregation deployments.
// The HPA scales prefill and decode LWS replicas based on EPP metrics.
type PDHPAConfig struct {
	// Enabled controls whether HPA is created for prefill and decode LWS.
	// Default: true (when PDConfig.HPA is nil, HPA is enabled by default)
	Enabled bool `json:"enabled"`

	// MinReplicas is the minimum number of LWS replicas (leader pods) for both prefill and decode.
	// The HPA will never scale below this value.
	// Default: 1
	MinReplicas int `json:"min_replicas,omitempty"`

	// MaxReplicas is the maximum number of LWS replicas (leader pods) for both prefill and decode.
	// The HPA will never scale above this value.
	// Default: maxReplica from request, or 2 if not specified
	MaxReplicas int `json:"max_replicas,omitempty"`

	// QueueThreshold is the EPP pending queue size threshold for scale-up decisions.
	// When the average queue size across all decode pods exceeds this value,
	// the HPA will scale up decode replicas.
	// Default: 3
	QueueThreshold int `json:"queue_threshold,omitempty"`

	// RunningThreshold is the EPP running requests threshold for scale-up decisions.
	// When the average number of running requests per decode pod exceeds this value,
	// the HPA will scale up decode replicas.
	// Default: 100
	RunningThreshold int `json:"running_threshold,omitempty"`

	// ScaleUpCooldown is the stabilization window in seconds for scale-up decisions.
	// The HPA will only scale up after the metric exceeds the threshold for this duration,
	// preventing scaling on transient spikes.
	// Default: 60 (1 minute)
	ScaleUpCooldown int `json:"scale_up_cooldown,omitempty"`

	// ScaleDownCooldown is the stabilization window in seconds for scale-down decisions.
	// The HPA will not scale down within this window after a scale-up event,
	// preventing flapping during transient load changes.
	// Default: 300 (5 minutes)
	ScaleDownCooldown int `json:"scale_down_cooldown,omitempty"`
}

// ApplyDefaults fills in default values for PDConfig fields.
// PrefillReplicas defaults to minReplica, then 1.
// DecodeReplicas defaults to minReplica, then 1.
// HPA defaults to enabled when nil, with sensible defaults.
func (p *PDConfig) ApplyDefaults(minReplica int, maxReplica int) {
	if p.PrefillReplicas == 0 {
		if minReplica > 0 {
			p.PrefillReplicas = minReplica
		} else {
			p.PrefillReplicas = 1
		}
	}
	if p.DecodeReplicas == 0 {
		if minReplica > 0 {
			p.DecodeReplicas = minReplica
		} else {
			p.DecodeReplicas = 1
		}
	}
	if p.HPA == nil {
		p.HPA = &PDHPAConfig{
			Enabled: true,
		}
	}
	p.HPA.ApplyDefaults(maxReplica)
}

// ApplyDefaults fills in default values for PDHPAConfig fields.
// MinReplicas defaults to 1.
// MaxReplicas defaults to maxReplica, then 2.
// QueueThreshold defaults to 3.
// RunningThreshold defaults to 100.
// ScaleUpCooldown defaults to 60.
// ScaleDownCooldown defaults to 300.
func (h *PDHPAConfig) ApplyDefaults(maxReplica int) {
	if h.MinReplicas == 0 {
		h.MinReplicas = 1
	}
	if h.MaxReplicas == 0 {
		if maxReplica > 0 {
			h.MaxReplicas = maxReplica
		} else {
			h.MaxReplicas = 2
		}
	}
	if h.QueueThreshold == 0 {
		h.QueueThreshold = 3
	}
	if h.RunningThreshold == 0 {
		h.RunningThreshold = 100
	}
	if h.ScaleUpCooldown == 0 {
		h.ScaleUpCooldown = 60
	}
	if h.ScaleDownCooldown == 0 {
		h.ScaleDownCooldown = 300
	}
}

// PDPlanResult holds the auto-planned PD disaggregation configuration for one role (prefill or decode).
type PDPlanResult struct {
	// TP is the tensor parallelism degree.
	TP int
	// EP is the expert parallelism degree (1 for dense models).
	EP int
	// DP is the data parallelism degree (number of replicas).
	// TotalGPUs = TP * EP * DP.
	DP int
	// TotalGPUs is the total number of GPUs per LWS group (TP * EP * DP).
	TotalGPUs int
	// LWSSize is the number of pods (workers) in the LWS group.
	LWSSize int
	// GPUsPerPod is the number of GPUs requested per pod.
	GPUsPerPod int
	// WeightMemPerGPU is the estimated model weight memory per GPU in GB.
	WeightMemPerGPU float64
	// RemainingVRAMForKV is the estimated remaining VRAM per GPU for KV cache in GB.
	RemainingVRAMForKV float64
}

// PDModelSpec describes a model's parameter layout for PD planning.
type PDModelSpec struct {
	// TotalParamsB is the total parameter count in billions (e.g., 671.0 for DeepSeek-V3).
	TotalParamsB float64
	// NonMoEParamsB is the non-expert (shared attention + shared expert) parameter count in billions.
	NonMoEParamsB float64
	// TotalExperts is the number of routed experts (0 for dense models).
	TotalExperts int
	// Precision is the inference precision: "fp16", "bf16", "fp8", "int8", "int4".
	Precision string
}

// PDGPUConfig describes the GPU hardware for PD planning.
type PDGPUConfig struct {
	// VRAMGB is the VRAM capacity of a single GPU in GB.
	VRAMGB float64
	// GPUsPerNode is the number of GPUs per node (typically 8).
	GPUsPerNode int
}

// PDPlanInput holds all inputs for the PD planner.
type PDPlanInput struct {
	Model          PDModelSpec
	GPU            PDGPUConfig
	KVRatioPrefill float64
	KVRatioDecode  float64
	// MaxTotalGPUs limits the total GPUs per LWS group. 0 means no limit.
	// Practical deployments typically cap at 32 (4 nodes of 8 GPUs).
	MaxTotalGPUs int
}
