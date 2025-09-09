package types

import (
	"strings"
)

// ExtractBuildId JointSpaceNameBuildId reverse JointSpaceNameBuildId function to extract build_id
func ExtractBuildId(joint string) string {
	parts := strings.Split(joint, "_")
	if len(parts) < 3 {
		return ""
	}
	return parts[len(parts)-1]
}

type CMConfig struct {
	Namespace   string
	CmName      string
	DataKey     string
	FileContent []byte
}

type FileCMConfig struct {
	FileName      string // file name
	ConfigMapName string // configMap ID
	VolumeName    string // volume name
	ReadOnly      bool
}

var ConfigMapFiles = []FileCMConfig{
	{
		FileName:      "init.sh",
		ConfigMapName: "init-configmap",
		VolumeName:    "init-volume",
		ReadOnly:      false,
	},
	{
		FileName:      "Dockerfile-python3.10",
		ConfigMapName: "cpu-docker-configmap",
		VolumeName:    "cpu-docker-volume",
		ReadOnly:      true,
	},
	{
		FileName:      "Dockerfile-python3.10-cuda11.8.0",
		ConfigMapName: "gpu-docker-configmap",
		VolumeName:    "gpu-docker-volume",
		ReadOnly:      true,
	},
	{
		FileName:      "Dockerfile-python3.10-cuda12.1.0",
		ConfigMapName: "gpu-cu121-configmap",
		VolumeName:    "gpu-cu121-volume",
		ReadOnly:      true,
	},
	{
		FileName:      "Dockerfile-nginx",
		ConfigMapName: "nginx-docker-configmap",
		VolumeName:    "nginx-docker-volume",
		ReadOnly:      true,
	},
}
