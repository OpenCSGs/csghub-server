package deploy

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/reporter"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/common/config"
)

var (
	fifoScheduler   scheduler.Scheduler
	defaultDeployer Deployer
)

func Init(c common.DeployConfig, config *config.Config) error {
	// ib := imagebuilder.NewLocalBuilder()
	ib, err := imagebuilder.NewRemoteBuilder(c.ImageBuilderURL, c)
	if err != nil {
		panic(fmt.Errorf("failed to create image builder:%w", err))
	}
	ir, err := imagerunner.NewRemoteRunner(c.ImageRunnerURL, c)
	if err != nil {
		panic(fmt.Errorf("failed to create image runner:%w", err))
	}

	logReporter, err := reporter.NewAndStartLogCollector(context.TODO(), config, types.ClientTypeCSGHUB)
	if err != nil {
		return fmt.Errorf("failed to create log reporter:%w", err)
	}

	fifoScheduler, err = scheduler.NewFIFOScheduler(ib, ir, c, logReporter)
	if err != nil {
		return fmt.Errorf("failed to create scheduler:%w", err)
	}

	deployer, err := newDeployer(fifoScheduler, ib, ir, c, logReporter, config)
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
