package deploy

import (
	"fmt"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
)

var (
	fifoScheduler   scheduler.Scheduler
	defaultDeployer Deployer
)

func Init(c common.DeployConfig) error {
	// ib := imagebuilder.NewLocalBuilder()
	ib, err := imagebuilder.NewRemoteBuilder(c.ImageBuilderURL)
	if err != nil {
		panic(fmt.Errorf("failed to create image builder:%w", err))
	}
	ir, err := imagerunner.NewRemoteRunner(c.ImageRunnerURL)
	if err != nil {
		panic(fmt.Errorf("failed to create image runner:%w", err))
	}

	fifoScheduler = scheduler.NewFIFOScheduler(ib, ir, c)
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
