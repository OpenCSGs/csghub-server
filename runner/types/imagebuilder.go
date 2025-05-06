package types

import (
	"fmt"
)

// ImagebuilderStatusRes represents build status response
// @Schema
type ImagebuilderStatusRes struct {
	WorkName string `json:"work_name"`
	Status   string `json:"status"`
	Message  string `json:"message"` // Optional: Additional message or error details
}

// SpaceBuilderConfig defines image build parameters
// @Schema
type SpaceBuilderConfig struct {
	ClusterID      string `json:"cluster_id"`
	SpaceName      string `json:"space_name"`
	OrgName        string `json:"org_name"`
	SpaceURL       string `json:"space_url"`
	Sdk            string `json:"sdk"`
	Sdk_version    string `json:"sdk_version"`
	PythonVersion  string `json:"python_version"`
	Hardware       string `json:"hardware,omitempty"`
	FactoryBuild   bool   `json:"factory_build,omitempty"`
	GitRef         string `json:"git_ref"`
	UserId         string `json:"user_id"`
	GitAccessToken string `json:"git_access_token"`
	BuildId        string `json:"build_id,omitempty"`
}

func JointSpaceNameBuildId(spaceName, name, build_id string) string {
	return fmt.Sprintf("%s_%s_%s", spaceName, name, build_id)
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
		FileName:      "Dockerfile-nginx",
		ConfigMapName: "nginx-docker-configmap",
		VolumeName:    "nginx-docker-volume",
		ReadOnly:      true,
	},
}

type ImageBuilderWork struct {
	WorkName   string `bun:"work_name,notnull,unique" json:"work_name"`
	WorkStatus string `bun:"work_status,notnull" json:"work_status"`
	Message    string `bun:"message" json:"message"`

	ImagePath string `bun:"image_path,notnull" json:"image_path"`

	BuildId string `bun:"build_id,notnull,unique" json:"build_id"`
	Log     string `bun:"log" json:"log"`
}
