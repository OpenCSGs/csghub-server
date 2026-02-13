package types

const (
	KnativeConfigLabelName  = "serving.knative.dev/service"
	WorkflowConfigLabelName = "workflows.argoproj.io/workflow"
	UserContainerName       = "user-container"
	InitRepoName            = "init-repo"
	MainContainerName       = "main"
)

var LogTargetContainersMap = map[string]struct{}{
	UserContainerName: {},
	InitRepoName:      {},
	MainContainerName: {},
}
