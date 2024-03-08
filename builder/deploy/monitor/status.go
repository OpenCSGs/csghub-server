package monitor

// sub build task status
const (
	BuildPending    = 0
	BuildInProgress = 1
	BuildFailed     = 2
	BuildSucceed    = 3
)

// sub run task status
const (
	PrepareToRun = 0
	StartUp      = 1
	Running      = 2
	RunTimeError = 3
)

const (
	DeployBuildPending    = 10
	DeployBuildInProgress = 11
	DeployBuildFailed     = 12
	DeployBuildSucceed    = 13

	DeployPrepareToRun = 20
	DeployStartUp      = 21
	DeployRunning      = 22
	DeployRunTimeError = 23
)
