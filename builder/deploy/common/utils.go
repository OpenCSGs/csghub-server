package common

import (
	"fmt"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

func GetNamespaceAndNameFromGitPath(gitpath string) (string, string, error) {
	if gitpath == "" {
		return "", "", fmt.Errorf("empty git path %s", gitpath)
	}
	var fields []string
	idx := strings.Index(gitpath, "_")
	if idx > -1 && idx+1 < len(gitpath) {
		fields = strings.Split(gitpath[idx+1:], "/")
		if len(fields) != 2 {
			return "", "", fmt.Errorf("empty git path %s", gitpath)
		}
	} else {
		return "", "", fmt.Errorf("empty git path %s", gitpath)
	}
	return fields[0], fields[1], nil
}

func GetValidSceneType(deployType int) types.SceneType {
	switch deployType {
	case types.SpaceType:
		return types.SceneSpace
	case types.InferenceType:
		return types.SceneModelInference
	case types.FinetuneType:
		return types.SceneModelFinetune
	case types.ServerlessType:
		return types.SceneModelServerless
	default:
		return types.SceneUnknow
	}
}

func UpdateEvaluationEnvHardware(env map[string]string, hardware types.HardWare) {
	xpuNum := "0"
	if hardware.Gpu.Num != "" {
		env["GPU_NUM"] = hardware.Gpu.Num
		xpuNum = hardware.Gpu.Num
	} else if hardware.Npu.Num != "" {
		env["NPU_NUM"] = hardware.Npu.Num
		xpuNum = hardware.Npu.Num
	} else if hardware.Gcu.Num != "" {
		env["GCU_NUM"] = hardware.Gcu.Num
		xpuNum = hardware.Gcu.Num
	} else if hardware.Mlu.Num != "" {
		env["MLU_NUM"] = hardware.Mlu.Num
		xpuNum = hardware.Mlu.Num
	} else if hardware.Dcu.Num != "" {
		env["DCU_NUM"] = hardware.Dcu.Num
		xpuNum = hardware.Dcu.Num
	} else if hardware.GPGpu.Num != "" {
		env["GPGPU_NUM"] = hardware.GPGpu.Num
		xpuNum = hardware.GPGpu.Num
	}
	if xpuNum != "0" {
		env["GPU_NUM"] = xpuNum
	}
}

func ResourceType(hardware types.HardWare) types.ResourceType {
	resourceType := types.ResourceTypeCPU
	if hardware.Gpu.Num != "" {
		resourceType = types.ResourceTypeGPU
	} else if hardware.Npu.Num != "" {
		resourceType = types.ResourceTypeNPU
	} else if hardware.Mlu.Num != "" {
		resourceType = types.ResourceTypeMLU
	} else if hardware.Gcu.Num != "" {
		resourceType = types.ResourceTypeGCU
	} else if hardware.Dcu.Num != "" {
		resourceType = types.ResourceTypeDCU
	} else if hardware.GPGpu.Num != "" {
		resourceType = types.ResourceTypeGPGPU
	}
	return resourceType
}

func GetResourceAndType(hardware types.HardWare) (string, string) {
	resource := hardware.Cpu.Num + "vCPU·" + hardware.Memory
	resourceType := ""
	if hardware.Gpu.Num != "" {
		resourceType = hardware.Gpu.Type
		resource += "·" + hardware.Gpu.Num + "GPU"
	} else if hardware.Npu.Num != "" {
		resourceType = hardware.Npu.Type
		resource += "·" + hardware.Npu.Num + "NPU"
	} else if hardware.Mlu.Num != "" {
		resourceType = hardware.Mlu.Type
		resource += "·" + hardware.Mlu.Num + "Mlu"
	} else if hardware.Gcu.Num != "" {
		resourceType = hardware.Gcu.Type
		resource += "·" + hardware.Gcu.Num + "GCU"
	} else if hardware.Dcu.Num != "" {
		resourceType = hardware.Dcu.Type
		resource += "·" + hardware.Dcu.Num + "Dcu"
	} else {
		resourceType = hardware.Cpu.Type
	}
	return resource, resourceType
}

func ContainsGraphicResource(hardware types.HardWare) bool {
	if hardware.Gpu.Num != "" || hardware.Npu.Num != "" ||
		hardware.Gcu.Num != "" || hardware.Mlu.Num != "" ||
		hardware.Dcu.Num != "" || hardware.GPGpu.Num != "" {
		return true
	}
	return false
}

func GetXPUNumber(hardware types.HardWare) (int, error) {
	var xpuNumStr = "0"
	if hardware.Gpu.Num != "" {
		xpuNumStr = hardware.Gpu.Num
	} else if hardware.Npu.Num != "" {
		xpuNumStr = hardware.Npu.Num
	} else if hardware.Gcu.Num != "" {
		xpuNumStr = hardware.Gcu.Num
	} else if hardware.Mlu.Num != "" {
		xpuNumStr = hardware.Mlu.Num
	} else if hardware.Dcu.Num != "" {
		xpuNumStr = hardware.Dcu.Num
	} else if hardware.GPGpu.Num != "" {
		xpuNumStr = hardware.GPGpu.Num
	}
	xpuNum, err := strconv.Atoi(xpuNumStr)
	return xpuNum, err
}
