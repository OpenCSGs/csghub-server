package common

import "time"

type DeployConfig struct {
	ImageBuilderURL         string
	ImageRunnerURL          string
	MonitorInterval         time.Duration
	InternalRootDomain      string
	SpaceDeployTimeoutInMin int
	ModelDeployTimeoutInMin int
	ModelDownloadEndpoint   string
	PublicRootDomain        string
	SSHDomain               string
	//download lfs object from internal s3 address
	S3Internal bool
}
