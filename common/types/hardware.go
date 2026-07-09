package types

import (
	"encoding/json"
	"strings"
)

type ResourceReasonType string

const (
	AvailableTypeOK                     ResourceReasonType = "ok"
	UnAvailableTypeInvalidHardware      ResourceReasonType = "invalid_hardware"
	UnAvailableTypeInvalidXPUType       ResourceReasonType = "invalid_xpu_type"
	UnAvailableTypeInvalidCPUNum        ResourceReasonType = "invalid_cpu_num"
	UnAvailableTypeInvalidMemorySize    ResourceReasonType = "invalid_memory_size"
	UnAvailableTypeInvalidXPUNum        ResourceReasonType = "invalid_xpu_num"
	UnAvailableTypeInvalidXPUMemorySize ResourceReasonType = "invalid_xpu_memory_size"
	UnAvailableTypeInvalidXPUMemoryLoss ResourceReasonType = "invalid_xpu_memory_loss"
	UnAvailableTypeInsufficientCPU      ResourceReasonType = "insufficient_cpu"
	UnAvailableTypeInsufficientMemory   ResourceReasonType = "insufficient_memory"
	UnAvailableTypeInsufficientXPU      ResourceReasonType = "insufficient_xpu"
	UnAvailableTypeInsufficientVXPU     ResourceReasonType = "insufficient_vxpu"
	UnAvailableTypeEnableVXPU           ResourceReasonType = "enable_vxpu"
	UnAvailableTypeDisableVXPU          ResourceReasonType = "disable_vxpu"
	UnAvailableTypeDisableScheduling    ResourceReasonType = "disable_scheduling"
	UnAvailableTypeNodeOffline          ResourceReasonType = "node_offline"
	UnAvailableTypePriceUndefined       ResourceReasonType = "price_undefined"
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
		Gpu              Processor `json:"gpu,omitempty"`   // nvidia,amd
		Npu              Processor `json:"npu,omitempty"`   // ascend
		Gcu              Processor `json:"gcu,omitempty"`   // enflame
		Mlu              Processor `json:"mlu,omitempty"`   // cambricon
		Dcu              Processor `json:"dcu,omitempty"`   // hygon
		GPGpu            Processor `json:"gpgpu,omitempty"` // iluvatar,metax
		Tpu              Processor `json:"tpu,omitempty"`   // chipltech
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

func (h *HardWare) GetResXPUMode() string {
	var xpuProc Processor

	switch {
	case strings.TrimSpace(h.Gpu.Num) != "":
		xpuProc = h.Gpu
	case strings.TrimSpace(h.Npu.Num) != "":
		xpuProc = h.Npu
	case strings.TrimSpace(h.Gcu.Num) != "":
		xpuProc = h.Gcu
	case strings.TrimSpace(h.Mlu.Num) != "":
		xpuProc = h.Mlu
	case strings.TrimSpace(h.Dcu.Num) != "":
		xpuProc = h.Dcu
	case strings.TrimSpace(h.GPGpu.Num) != "":
		xpuProc = h.GPGpu
	case strings.TrimSpace(h.Tpu.Num) != "":
		xpuProc = h.Tpu
	}

	xpuModel := strings.TrimSpace(xpuProc.Type)

	return xpuModel

}

func parseHardWare(res string) (HardWare, error) {
	var hardware HardWare
	err := json.Unmarshal([]byte(res), &hardware)
	if err != nil {
		return HardWare{}, err
	}

	return hardware, nil
}

func GetResXPUMode(res string) (string, error) {
	hardware, err := parseHardWare(res)
	if err != nil {
		return "", err
	}

	return hardware.GetResXPUMode(), nil
}

func GetHardwareType(res string) (string, error) {
	hardware, err := parseHardWare(res)
	if err != nil {
		return "", err
	}
	xpuType := hardware.GetResXPUMode()
	if len(xpuType) > 0 {
		return xpuType, nil
	}
	cpuType := hardware.Cpu.Type
	return cpuType, nil
}
