package types

import (
	"github.com/go-playground/validator/v10"
)

type ResourceType string
type PayMode string

const (
	ResourceTypeCPU   ResourceType = "cpu"
	ResourceTypeGPU   ResourceType = "gpu"
	ResourceTypeNPU   ResourceType = "npu"
	ResourceTypeGCU   ResourceType = "gcu"
	ResourceTypeGPGPU ResourceType = "gpgpu"
	ResourceTypeMLU   ResourceType = "mlu"
	ResourceTypeDCU   ResourceType = "dcu"
	ResourceTypeTPU   ResourceType = "tpu"
	PayModeFree       PayMode      = "free"
	PayModeMinute     PayMode      = "minute"
	PayModeMonth      PayMode      = "month"
	PayModeYear       PayMode      = "year"
)

// HardwareType is a bitmask value identifying a hardware category. Each category
// occupies one bit so a constraint's required_hardware can express "any of these
// categories" via bitwise OR. Used by the space_resource_scenario_constraints table.
//
// Bit assignment mirrors the ResourceType string enum above (cpu=bit0, gpu=bit1, ...).
type HardwareType int64

const (
	HardwareCPU   HardwareType = 1 << 0
	HardwareGPU   HardwareType = 1 << 1
	HardwareNPU   HardwareType = 1 << 2
	HardwareGCU   HardwareType = 1 << 3
	HardwareGPGPU HardwareType = 1 << 4
	HardwareMLU   HardwareType = 1 << 5
	HardwareDCU   HardwareType = 1 << 6
	HardwareTPU   HardwareType = 1 << 7
)

// HardwareMaskGraphic matches any graphic accelerator (gpu/npu/gcu/gpgpu/mlu/
// dcu/tpu), i.e. everything except pure CPU. Mirrors the legacy
// ContainsGraphicResource semantics used by the finetune deploy availability check.
const HardwareMaskGraphic HardwareType = HardwareGPU | HardwareNPU | HardwareGCU |
	HardwareGPGPU | HardwareMLU | HardwareDCU | HardwareTPU

// HardwareToMask returns the bitmask of every hardware category present in the
// given hardware spec (every XPU/CPU with a non-empty Num). Used to test a
// resource against a space_resource_scenario_constraints.required_hardware bitmask via
// `HardwareToMask(hardware) & requiredHardware <> 0`.
func HardwareToMask(hardware HardWare) HardwareType {
	var mask HardwareType
	if hardware.Cpu.Num != "" {
		mask |= HardwareCPU
	}
	if hardware.Gpu.Num != "" {
		mask |= HardwareGPU
	}
	if hardware.Npu.Num != "" {
		mask |= HardwareNPU
	}
	if hardware.Gcu.Num != "" {
		mask |= HardwareGCU
	}
	if hardware.GPGpu.Num != "" {
		mask |= HardwareGPGPU
	}
	if hardware.Mlu.Num != "" {
		mask |= HardwareMLU
	}
	if hardware.Dcu.Num != "" {
		mask |= HardwareDCU
	}
	if hardware.Tpu.Num != "" {
		mask |= HardwareTPU
	}
	return mask
}

// HardwareSatisfiesConstraint reports whether the given hardware meets a
// scenario's hardware constraints. It is the shared implementation used by both
// the space resource Index query and the sandbox auto-allocator so the two paths
// apply identical rules. Two independent checks:
//   - required ("at least one of"): the hardware mask must share at least one bit
//     with required (mask & required != 0). A zero required means no "must have"
//     requirement. e.g. finetune requires a graphic accelerator.
//   - exclude ("none of"): the hardware mask must share NO bit with exclude
//     (mask & exclude == 0). A zero exclude means nothing excluded. e.g. sandbox
//     excludes all graphic accelerators to enforce "pure CPU" — required can only
//     say "has CPU", not "only CPU", so exclude is what blocks a CPU+GPU resource.
//
// Callers with no configured constraint pass required=0 and exclude=0, which
// always satisfies.
func HardwareSatisfiesConstraint(required, exclude int64, hardware HardWare) bool {
	mask := int64(HardwareToMask(hardware))
	// "at least one of" — zero required means no must-have requirement.
	if required != 0 && mask&required == 0 {
		return false
	}
	// "none of" — zero exclude means nothing excluded.
	if exclude != 0 && mask&exclude != 0 {
		return false
	}
	return true
}

// ReplicaSatisfiesConstraint reports whether the resource replica count is within
// the scenario's max_replica. It is the shared implementation used by both the
// space resource Index query and the sandbox auto-allocator so the two paths
// apply identical rules. A zero maxReplica means "unlimited" (always satisfied),
// matching the convention used elsewhere for optional caps.
func ReplicaSatisfiesConstraint(maxReplica, replicas int) bool {
	if maxReplica == 0 {
		return true
	}
	return replicas <= maxReplica
}

// ScenarioType is a bitmask value identifying which deploy/workflow scenarios a
// space resource supports. Each scenario occupies a single bit (power of two),
// so a resource can support multiple scenarios at once via bitwise OR.
// Use ScenarioAll (-1, all bits set) to express "supports every scenario";
// new scenarios added later are covered automatically.
type ScenarioType int64

// ScenarioAll means the resource supports every known scenario. It is the
// two's-complement -1 (all 64 bits set), so any non-zero query mask matches and
// scenarios added in the future are covered without maintenance.
const ScenarioAll ScenarioType = -1

// ScenarioName* constants are the machine names of each scenario, stored in the
// space_resource_scenario_constraints table and used by the mask<->name conversion.
// Defining them as constants (instead of string literals) keeps the seed, the
// scenarioBit table, and any caller that needs a name in sync.
const (
	ScenarioNameSpace            = "space"
	ScenarioNameInference        = "inference"
	ScenarioNameFinetune         = "finetune"
	ScenarioNameServerless       = "serverless"
	ScenarioNameNotebook         = "notebook"
	ScenarioNameSandbox          = "sandbox"
	ScenarioNameWfEvaluation     = "wf_evaluation"
	ScenarioNameWfClawEval       = "wf_claw_eval"
	ScenarioNameWfFinetune       = "wf_finetune"
	ScenarioNameWfDataflow       = "wf_dataflow"
	ScenarioNameWfDataflowLLMLog = "wf_dataflow_llmlog"
)

// scenarioBit maps each scenario bit constant to its name. These bit numbers
// are RESERVED position assignments — once a scenario is assigned a bit it must
// not change (the bitmask stored on space_resources.scenarios depends on it), so
// the list only ever grows. It is used by ScenarioName to resolve a bit constant
// (e.g. ScenarioFinetune) into its name string when seeding the
// space_resource_scenario_constraints table.
//
// The scenario CATALOG (name, code, category, display_name) and the per-scenario
// filtering constraints live in the space_resource_scenario_constraints table at
// runtime — that table is the source of truth for the /scenarios listing API and
// the mask<->name conversions. This Go table exists solely to seed that DB table
// and to reserve/document the bit positions; adding a scenario at runtime does
// NOT require changing this file (only insert a DB row with the next free bit).
//
// The 64-bit space is split in half for future growth:
//   - bits 0-31:  deploy scenarios (DeployType, see common/types/model.go). The
//     bit position equals the DeployType int value (SpaceType=0 ... SandboxType=7)
//     so the deploy_type passed to the Index API doubles as a scenario code.
//   - bits 32-63: workflow scenarios.
//
// Note: deploy Finetune and workflow Finetune are distinct concepts and therefore
// occupy different bits.
var scenarioBit = []struct {
	name string
	bit  ScenarioType
}{
	{ScenarioNameSpace, 1 << 0},             // bit0  deploy Space
	{ScenarioNameInference, 1 << 1},         // bit1  deploy Inference
	{ScenarioNameFinetune, 1 << 2},          // bit2  deploy Finetune
	{ScenarioNameServerless, 1 << 3},        // bit3  deploy Serverless
	{ScenarioNameNotebook, 1 << 5},          // bit5  deploy Notebook
	{ScenarioNameSandbox, 1 << 7},           // bit7  deploy Sandbox
	{ScenarioNameWfEvaluation, 1 << 32},     // bit32 workflow Evaluation
	{ScenarioNameWfClawEval, 1 << 33},       // bit33 workflow ClawEval
	{ScenarioNameWfFinetune, 1 << 37},       // bit37 workflow Finetune
	{ScenarioNameWfDataflow, 1 << 38},       // bit38 workflow Dataflow
	{ScenarioNameWfDataflowLLMLog, 1 << 39}, // bit39 workflow LLM Log Dataflow
}

const (
	// deploy scenarios (bits 0-31)
	ScenarioSpace      ScenarioType = 1 << 0
	ScenarioInference  ScenarioType = 1 << 1
	ScenarioFinetune   ScenarioType = 1 << 2
	ScenarioServerless ScenarioType = 1 << 3
	ScenarioNotebook   ScenarioType = 1 << 5
	ScenarioSandbox    ScenarioType = 1 << 7
	// workflow scenarios (bits 32-63)
	ScenarioWfEvaluation     ScenarioType = 1 << 32
	ScenarioWfClawEval       ScenarioType = 1 << 33
	ScenarioWfFinetune       ScenarioType = 1 << 37
	ScenarioWfDataflow       ScenarioType = 1 << 38
	ScenarioWfDataflowLLMLog ScenarioType = 1 << 39
)

// ScenarioName returns the name of the scenario identified by the given single-bit
// ScenarioType value (e.g. ScenarioFinetune => "finetune"). It is used when seeding
// the space_resource_scenario_constraints table. It returns "" when the value is
// not exactly one known scenario bit: the zero value, ScenarioAll (all bits set),
// a multi-bit mask, or a bit not assigned to any scenario.
func ScenarioName(s ScenarioType) string {
	for _, sc := range scenarioBit {
		if sc.bit == s {
			return sc.name
		}
	}
	return ""
}

// ScenarioCategory groups scenarios by origin.
type ScenarioCategory string

const (
	ScenarioCategoryDeploy   ScenarioCategory = "deploy"
	ScenarioCategoryWorkflow ScenarioCategory = "workflow"
)

// ScenarioInfo describes a single scenario for the public scenario listing API
// (GET /space_resources/scenarios). The data is read from the
// space_resource_scenario_constraints table at runtime.
//
//   - Code: bit position (0-7 for deploy, 32-39 for workflow); it doubles as the
//     value callers pass to the space resource Index API as deploy_type. The
//     bitmask is `1 << Code`. Deploy codes equal the DeployType int values.
//   - Name: scenario name (machine name, e.g. "finetune").
//   - I18nKey: i18n key (e.g. "scenario.finetune") resolved by the frontend
//     via $t; the localized text lives in the frontend locale files.
//   - Category: "deploy" or "workflow".
//   - RequiredHardware/ExcludeHardware/MaxReplica: the scenario's hardware
//     constraints, exposed so the frontend can pre-validate a resource's
//     hardware against the scenario (e.g. grey out sandbox for a CPU+GPU
//     resource) before submitting. Semantics match HardwareSatisfiesConstraint
//     and ReplicaSatisfiesConstraint.
type ScenarioInfo struct {
	Code             int              `json:"code"`
	Name             string           `json:"name"`
	I18nKey          string           `json:"i18n_key"`
	Category         ScenarioCategory `json:"category"`
	RequiredHardware int64            `json:"required_hardware"`
	ExcludeHardware  int64            `json:"exclude_hardware"`
	MaxReplica       int              `json:"max_replica"`
}

func ResourceTypeValid(resourceType ResourceType) bool {
	return resourceType == ResourceTypeCPU ||
		resourceType == ResourceTypeGPU ||
		resourceType == ResourceTypeNPU ||
		resourceType == ResourceTypeGCU ||
		resourceType == ResourceTypeGPGPU ||
		resourceType == ResourceTypeMLU ||
		resourceType == ResourceTypeDCU ||
		resourceType == ResourceTypeTPU
}

var ResourceTypeValidator validator.Func = func(fl validator.FieldLevel) bool {
	return ResourceTypeValid(ResourceType(fl.Field().String()))
}

type SpaceResource struct {
	ID                  int64                     `json:"id"`
	Name                string                    `json:"name"`
	ClusterID           string                    `json:"cluster_id"`
	ClusterRegion       string                    `json:"cluster_region"`
	Resources           string                    `json:"resources"`
	Price               float64                   `json:"price"`
	PriceUnit           int64                     `json:"price_unit"`
	PriceUnitType       SkuUnitType               `json:"price_unit_type"`
	IsAvailable         bool                      `json:"is_available"`
	Type                ResourceType              `json:"type"`
	PayMode             PayMode                   `json:"pay_mode"`
	IsReserved          bool                      `json:"is_reserved"`
	OrderDetailId       int64                     `json:"order_detail_id"`
	AvailableStatusList []ResourceAvailableStatus `json:"available_status_list"`
	PriceUndefined      bool                      `json:"price_undefined"`
	Scenarios           []string                  `json:"scenarios"`
}

type CreateSpaceResourceReq struct {
	Name      string   `json:"name" binding:"required"`
	Resources string   `json:"resources" binding:"required"`
	ClusterID string   `json:"cluster_id" binding:"required"`
	Scenarios []string `json:"scenarios"`
}

type UpdateSpaceResourceReq struct {
	ID        int64    `json:"-"`
	Name      string   `json:"name"`
	Resources string   `json:"resources"`
	Scenarios []string `json:"scenarios"`
}

type SpaceResourceIndexReq struct {
	ClusterIDs   []string     `json:"cluster_id" form:"cluster_id"`
	DeployType   int          `json:"deploy_type" form:"deploy_type" binding:"omitempty"`
	CurrentUser  string       `json:"current_user" form:"current_user"`
	ResourceType ResourceType `json:"resource_type" form:"resource_type" binding:"omitempty,resource_type"`
	HardwareType string       `json:"hardware_type" form:"hardware_type"`
	IsAvailable  *bool        `json:"is_available" form:"is_available"`
	Per          int          `json:"per" form:"per,default=100" binding:"min=1,max=100"`
	Page         int          `json:"page" form:"page,default=1" binding:"min=1"`
}

type SpaceResourceFilter struct {
	ClusterID    string       `json:"cluster_id"`
	ResourceType ResourceType `json:"resource_type"`
	HardwareType string       `json:"hardware_type"`
}
