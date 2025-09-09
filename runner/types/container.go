package types

const (
	UserContainerName = "user-container"
	InitRepoName      = "init-repo"
	MainContainerName = "main"
)

var LogTargetContainersMap = map[string]struct{}{
	UserContainerName: {},
	InitRepoName:      {},
	MainContainerName: {},
}
