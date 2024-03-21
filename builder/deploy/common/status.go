package common

import "strings"

// sub build task status

const (
	Pending = 0
	// step one: build
	Building     = 11
	BuildFailed  = 12
	BuildSuccess = 13
	// step two: deploy and run
	Deploying    = 20
	DeployFailed = 21
	Startup      = 22
	Running      = 23
	RunTimeError = 24
	Sleeping     = 25
)

func ImageIDToServiceName(imageID string) string {
	return strings.ReplaceAll(strings.ReplaceAll(imageID, ":", "-"), ".", "-")
}
