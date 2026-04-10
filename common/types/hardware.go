package types

type ResourceReasonType string

const (
	AvailableTypeOK                     ResourceReasonType = "ok"
	UnAvailableTypeInvalidHardware      ResourceReasonType = "invalid_hardware"
	UnAvailableTypeInvalidXPUType       ResourceReasonType = "invalid_xpu_type"
	UnAvailableTypeInvalidCPUNum        ResourceReasonType = "invalid_cpu_num"
	UnAvailableTypeInvalidMemorySize    ResourceReasonType = "invalid_memory_size"
	UnAvailableTypeInvalidXPUNum        ResourceReasonType = "invalid_xpu_num"
	UnAvailableTypeInvalidXPUMemorySize ResourceReasonType = "invalid_xpu_memory_size"
	UnAvailableTypeInsufficientCPU      ResourceReasonType = "insufficient_cpu"
	UnAvailableTypeInsufficientMemory   ResourceReasonType = "insufficient_memory"
	UnAvailableTypeInsufficientXPU      ResourceReasonType = "insufficient_xpu"
	UnAvailableTypeInsufficientVXPU     ResourceReasonType = "insufficient_vxpu"
	UnAvailableTypeEnableVXPU           ResourceReasonType = "enable_vxpu"
	UnAvailableTypeDisableVXPU          ResourceReasonType = "disable_vxpu"
)

type (
	Processor struct {
		Type            string            `json:"type,omitempty"`
		Num             string            `json:"num,omitempty"`
		ResourceName    string            `json:"resource_name,omitempty"`
		Labels          map[string]string `json:"labels,omitempty"`
		ResourceMemName string            `json:"resource_mem_name,omitempty"`
		MemSize         string            `json:"mem_size,omitempty"` // MB
	}

	CPU struct {
		Type   string            `json:"type,omitempty"`
		Num    string            `json:"num,omitempty"`
		Labels map[string]string `json:"labels,omitempty"`
	}

	HardWare struct {
		Gpu              Processor `json:"gpu,omitempty"`   // nvidia
		Npu              Processor `json:"npu,omitempty"`   // ascend
		Gcu              Processor `json:"gcu,omitempty"`   // enflame
		Mlu              Processor `json:"mlu,omitempty"`   // cambricon
		Dcu              Processor `json:"dcu,omitempty"`   // hygon
		GPGpu            Processor `json:"gpgpu,omitempty"` // iluvatar,metax
		Cpu              CPU       `json:"cpu,omitempty"`
		Memory           string    `json:"memory,omitempty"`
		EphemeralStorage string    `json:"ephemeral_storage,omitempty"`
		Replicas         int       `json:"replicas,omitempty"`
	}

	ResourceAvailableStatus struct {
		Available bool               `json:"available"`
		NodeName  string             `json:"node_name"`
		Reason    ResourceReasonType `json:"reason"`
	}
)
