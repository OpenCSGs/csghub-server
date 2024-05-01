package types

type (
	GPU struct {
		Type         string            `json:"type,omitempty" protobuf:"bytes,1,opt,name=type"`
		Num          string            `json:"num,omitempty" protobuf:"varint,2,opt,name=num"`
		ResourceName string            `json:"resource_name,omitempty" protobuf:"bytes,3,opt,name=resource_name"`
		Labels       map[string]string `json:"labels,omitempty" protobuf:"bytes,4,rep,name=labels"`
	}

	CPU struct {
		Type   string            `json:"type,omitempty" protobuf:"bytes,1,opt,name=type"`
		Num    string            `json:"num,omitempty" protobuf:"varint,2,opt,name=num"`
		Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,3,rep,name=labels"`
	}

	HardWare struct {
		Gpu              GPU    `json:"gpu,omitempty" protobuf:"bytes,1,opt,name=gpu"`
		Cpu              CPU    `json:"cpu,omitempty" protobuf:"bytes,2,opt,name=cpu"`
		Memory           string `json:"memory,omitempty" protobuf:"bytes,3,opt,name=memory"`
		EphemeralStorage string `json:"ephemeral_storage,omitempty" protobuf:"bytes,4,opt,name=ephemeral_storage"`
	}
)
