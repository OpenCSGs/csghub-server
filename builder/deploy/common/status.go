package common

// sub build task status
const (
	TaskStatusCancelled       = -1
	TaskStatusBuildPending    = 0
	TaskStatusBuildInProgress = 1
	TaskStatusBuildFailed     = 2
	TaskStatusBuildSucceed    = 3
	TaskStatusBuildSkip       = 4 // export for other package
	TaskStatusBuildInQueue    = 5
)

// sub deploy task status
const (
	TaskStatusDeployPending      = 0
	TaskStatusDeploying          = 1
	TaskStatusDeployFailed       = 2
	TaskStatusDeployStartUp      = 3
	TaskStatusDeployRunning      = 4
	TaskStatusDeployRunTimeError = 5
)

// deploy status
const (
	Pending = 0
	// step one: build
	BuildInQueue = 10
	Building     = 11
	BuildFailed  = 12
	BuildSuccess = 13
	BuildSkip    = 14 // Used when the build step is skipped
	// step two: deploy and run
	Deploying    = 20
	DeployFailed = 21
	Startup      = 22
	Running      = 23
	RunTimeError = 24
	Sleeping     = 25
	Stopped      = 26
	Deleted      = 27 // end user trigger delete action for deploy
)
