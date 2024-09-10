package deploy

import (
	"fmt"
	"time"

	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
)

var (
	fifoScheduler   scheduler.Scheduler
	defaultDeployer Deployer
)

func Init(c DeployConfig) error {
	// ib := imagebuilder.NewLocalBuilder()
	ib, err := imagebuilder.NewRemoteBuilder(c.ImageBuilderURL)
	if err != nil {
		panic(fmt.Errorf("failed to create image builder:%w", err))
	}
	ir, err := imagerunner.NewRemoteRunner(c.ImageRunnerURL)
	if err != nil {
		panic(fmt.Errorf("failed to create image runner:%w", err))
	}

	fifoScheduler = scheduler.NewFIFOScheduler(ib, ir, c.SpaceDeployTimeoutInMin, c.ModelDeployTimeoutInMin, c.ModelDownloadEndpoint, c.PublicRootDomain)
	deployer, err := newDeployer(fifoScheduler, ib, ir)
	if err != nil {
		return fmt.Errorf("failed to create deployer:%w", err)
	}

	deployer.internalRootDomain = c.InternalRootDomain
	defaultDeployer = deployer
	return nil
}

func NewDeployer() Deployer {
	return defaultDeployer
}

type DeployConfig struct {
	ImageBuilderURL         string
	ImageRunnerURL          string
	MonitorInterval         time.Duration
	InternalRootDomain      string
	SpaceDeployTimeoutInMin int
	ModelDeployTimeoutInMin int
	ModelDownloadEndpoint   string
	PublicRootDomain        string
}
