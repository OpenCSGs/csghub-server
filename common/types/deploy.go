package types

import (
	"encoding/json"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	// EngineArgEnableToolCalling is the JSON key for enabling tool calling in engine_args.
	EngineArgEnableToolCalling = "enable-tool-calling"
	// EngineArgCustomOptions is the JSON key for extra CLI flags in engine_args.
	EngineArgCustomOptions = "custom-options"
)

func engineArgCLIIndicatesToolCalling(engineArgs string) bool {
	return strings.Contains(engineArgs, "--enable-auto-tool-choice") ||
		strings.Contains(engineArgs, "tool-call-parser")
}

// EngineArgToolCallingEnabled reports whether tool calling is enabled in deploy engine_args.
// engine_args are stored as JSON; vLLM uses "disable" as the default sentinel while
// SGLang uses "enable" as the default sentinel.
func EngineArgToolCallingEnabled(engineArgs, runtimeFramework string) bool {
	engineArgs = strings.TrimSpace(engineArgs)
	if engineArgs == "" {
		return false
	}

	var argValuesMap map[string]string
	if err := json.Unmarshal([]byte(engineArgs), &argValuesMap); err != nil {
		// Legacy CLI-style engine args.
		return engineArgCLIIndicatesToolCalling(engineArgs)
	}

	if value, ok := argValuesMap[EngineArgEnableToolCalling]; ok {
		switch value {
		case "false", "0", "", "disable":
			return false
		}

		defaultSentinel := "disable"
		if strings.Contains(strings.ToLower(runtimeFramework), "sglang") {
			defaultSentinel = "enable"
		}
		return value != defaultSentinel
	}

	if customOptions, ok := argValuesMap[EngineArgCustomOptions]; ok {
		return engineArgCLIIndicatesToolCalling(customOptions)
	}

	return false
}

type DeployReq struct {
	CurrentUser string `json:"current_user"`
	PageOpts
	RepoType    RepositoryType `json:"repo_type"`
	DeployType  int            `json:"deploy_type"`
	DeployTypes []int          `json:"deploy_types"`
	Status      []int          `json:"status"`
	Query       string         `json:"query"`
	StartTime   *time.Time     `json:"start_time,omitempty"`
	EndTime     *time.Time     `json:"end_time,omitempty"`
}

type ServiceEvent struct {
	ServiceName string     `json:"service_name"` // service name
	Status      int        `json:"status"`       // event status
	Endpoint    string     `json:"endpoint"`     // service endpoint
	Message     string     `json:"message"`      // event message
	Reason      string     `json:"reason"`       // event reason
	TaskID      int64      `json:"task_id"`      // task id
	ClusterNode string     `json:"cluster_node"` // cluster node name
	QueueName   string     `json:"queue_name"`   // queue name
	Instances   []Instance `json:"instances"`
}

type StatRunningDeploy struct {
	DeployNum int `json:"deploy_num"`
	CPUNum    int `json:"cpu_num"`
	GPUNum    int `json:"gpu_num"`
	NpuNum    int `json:"npu_num"`
	GcuNum    int `json:"gcu_num"`
	MluNum    int `json:"mlu_num"`
	DcuNum    int `json:"dcu_num"`
	GPGpuNum  int `json:"gpgpu_num"`
}

type ClusterDeployReq struct {
	ClusterID    string `json:"cluster_id"`
	ClusterNode  string `json:"cluster_node"`
	Status       int    `json:"status"`
	ResourceID   int    `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Search       string `json:"search"`
	Per          int    `json:"per"`
	Page         int    `json:"page"`
}

// DeployExtend Use common fields for storage deployment to simplify the process of adding a large number
// of repetitive fields to the request structure for each different scenario.
type DeployExtend struct {
	NodeAffinity *corev1.NodeAffinity `json:"node_affinity,omitempty"`
	Tolerations  []Toleration         `json:"tolerations,omitempty"`
	PD           *PDConfig            `json:"pd,omitempty"`
}

type DeployTimeRangeReq struct {
	PageOpts
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}
