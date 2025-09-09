package common

import (
	"time"

	"opencsg.com/csghub-server/builder/redis"
)

type DeployConfig struct {
	ImageBuilderURL         string
	ImageRunnerURL          string
	MonitorInterval         time.Duration
	InternalRootDomain      string
	SpaceDeployTimeoutInMin int
	ModelDeployTimeoutInMin int
	ModelDownloadEndpoint   string
	ChargingEnable          bool
	PublicRootDomain        string
	RedisLocker             *redis.DistributedLocker
	UniqueServiceName       string
	//download lfs object from internal s3 address
	S3Internal         bool
	APIToken           string
	APIKey             string
	HeartBeatTimeInSec int
}
