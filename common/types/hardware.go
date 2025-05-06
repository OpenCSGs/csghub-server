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
		Gpu              Processor `json:"gpu,omitempty"`
		Npu              Processor `json:"npu,omitempty"`
		Enflame          Processor `json:"enflame,omitempty"`
		Mlu              Processor `json:"mlu,omitempty"`
		Cpu              CPU       `json:"cpu,omitempty"`
		Memory           string    `json:"memory,omitempty"`
		EphemeralStorage string    `json:"ephemeral_storage,omitempty"`
		Replicas         int       `json:"replicas,omitempty"`
	}
)
