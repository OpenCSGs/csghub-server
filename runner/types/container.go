package types

const (
	KnativeConfigLabelName  = "serving.knative.dev/service"
	WorkflowConfigLabelName = "workflows.argoproj.io/workflow"
	UserContainerName       = "user-container"
	InitRepoName            = "init-repo"
	MainContainerName       = "main"
	QueueProxyName          = "queue-proxy"
	Nvida_Com_GPU_Memory    = "nvidia.com/gpu.memory"
)

var LogTargetContainersMap = map[string]struct{}{
	UserContainerName: {},
	InitRepoName:      {},
	MainContainerName: {},
}
