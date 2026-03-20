package types

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type GeneratedResources struct {
	ResourceRequirements map[corev1.ResourceName]resource.Quantity
	NodeSelector         map[string]string
	NodeAffinity         *corev1.NodeAffinity
	Tolerations          []corev1.Toleration
}

type ResourceGeneratorParams struct {
	Hardware  types.HardWare
	Nodes     []types.Node
	DeployExt types.DeployExtend
	Config    *config.Config
}
