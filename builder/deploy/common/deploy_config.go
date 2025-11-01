package common

import (
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/redis"
	"opencsg.com/csghub-server/common/config"
)

type DeployConfig struct {
	ImageBuilderURL         string
	ImageRunnerURL          string
	MonitorInterval         time.Duration
	InternalRootDomain      string
	SpaceDeployTimeoutInMin int
	ModelDeployTimeoutInMin int
	BuildTimeoutInMin       int
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
	PublicDomain       string
	SSHDomain          string
}

func BuildDeployConfig(cfg *config.Config) DeployConfig {
	slog.Info("init distributed locker")
	redisLocker := redis.InitDistributedLocker(cfg)
	return DeployConfig{
		ImageBuilderURL:         cfg.Space.BuilderEndpoint,
		ImageRunnerURL:          cfg.Space.RunnerEndpoint,
		MonitorInterval:         10 * time.Second,
		InternalRootDomain:      cfg.Space.InternalRootDomain,
		SpaceDeployTimeoutInMin: cfg.Space.DeployTimeoutInMin,
		ModelDeployTimeoutInMin: cfg.Model.DeployTimeoutInMin,
		BuildTimeoutInMin:       cfg.Space.BuildTimeoutInMin,
		ModelDownloadEndpoint:   cfg.Model.DownloadEndpoint,
		ChargingEnable:          cfg.Accounting.ChargingEnable,
		PublicRootDomain:        cfg.Space.PublicRootDomain,
		S3Internal:              len(cfg.S3.InternalEndpoint) > 0,
		UniqueServiceName:       cfg.UniqueServiceName,
		APIToken:                cfg.APIToken,
		APIKey:                  cfg.APIToken,
		HeartBeatTimeInSec:      cfg.Runner.HearBeatIntervalInSec,
		PublicDomain:            cfg.APIServer.PublicDomain,
		SSHDomain:               cfg.APIServer.SSHDomain,
		RedisLocker:             redisLocker,
	}
}
