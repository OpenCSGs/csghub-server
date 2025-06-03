package types

type (
	Processor struct {
		Type         string            `json:"type,omitempty"`
		Num          string            `json:"num,omitempty"`
		ResourceName string            `json:"resource_name,omitempty"`
		Labels       map[string]string `json:"labels,omitempty"`
	}

	CPU struct {
		Type   string            `json:"type,omitempty"`
		Num    string            `json:"num,omitempty"`
		Labels map[string]string `json:"labels,omitempty"`
	}

	HardWare struct {
		Gpu              Processor `json:"gpu,omitempty"`   // nvidia
		Npu              Processor `json:"npu,omitempty"`   //ascend
		Gcu              Processor `json:"gcu,omitempty"`   // enflame
		Mlu              Processor `json:"mlu,omitempty"`   // cambricon
		Dcu              Processor `json:"dcu,omitempty"`   //hygon
		GPGpu            Processor `json:"gpgpu,omitempty"` // iluvatar,metax
		Cpu              CPU       `json:"cpu,omitempty"`
		Memory           string    `json:"memory,omitempty"`
		EphemeralStorage string    `json:"ephemeral_storage,omitempty"`
		Replicas         int       `json:"replicas,omitempty"`
	}
)
