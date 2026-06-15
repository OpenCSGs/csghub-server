package utils

import (
	"encoding/json"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

type VoucherMatchType string

const (
	VoucherMatchTypeBoth    VoucherMatchType = "both"
	VoucherMatchTypeXPU     VoucherMatchType = "xpu"
	VoucherMatchTypeCluster VoucherMatchType = "cluster"
	VoucherMatchTypeNone    VoucherMatchType = "none"
)

func GetResXPUMode(res string) (string, error) {
	var hardware types.HardWare
	err := json.Unmarshal([]byte(res), &hardware)
	if err != nil {
		return "", err
	}
	var xpuProc types.Processor

	switch {
	case strings.TrimSpace(hardware.Gpu.Num) != "":
		xpuProc = hardware.Gpu
	case strings.TrimSpace(hardware.Npu.Num) != "":
		xpuProc = hardware.Npu
	case strings.TrimSpace(hardware.Gcu.Num) != "":
		xpuProc = hardware.Gcu
	case strings.TrimSpace(hardware.Mlu.Num) != "":
		xpuProc = hardware.Mlu
	case strings.TrimSpace(hardware.Dcu.Num) != "":
		xpuProc = hardware.Dcu
	case strings.TrimSpace(hardware.GPGpu.Num) != "":
		xpuProc = hardware.GPGpu
	}

	xpuModel := strings.TrimSpace(xpuProc.Type)

	return xpuModel, nil
}
