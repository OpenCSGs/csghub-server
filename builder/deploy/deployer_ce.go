//go:build !ee && !saas

package deploy

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/bwmarrin/snowflake"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type deployer struct {
	scheduler    scheduler.Scheduler
	imageBuilder imagebuilder.Builder
	imageRunner  imagerunner.Runner

	deployTaskStore       database.DeployTaskStore
	spaceStore            database.SpaceStore
	spaceResourceStore    database.SpaceResourceStore
	runnerStatusCache     map[string]types.StatusResponse
	internalRootDomain    string
	snowflakeNode         *snowflake.Node
	eventPub              *event.EventPublisher
	runtimeFrameworkStore database.RuntimeFrameworksStore
	deployConfig          DeployConfig
	userStore             database.UserStore
}

func newDeployer(s scheduler.Scheduler, ib imagebuilder.Builder, ir imagerunner.Runner, c DeployConfig) (*deployer, error) {
	store := database.NewDeployTaskStore()
	node, err := snowflake.NewNode(1)
	if err != nil || node == nil {
		slog.Error("fail to generate uuid for inference service name", slog.Any("error", err))
		return nil, err
	}
	d := &deployer{
		scheduler:             s,
		imageBuilder:          ib,
		imageRunner:           ir,
		deployTaskStore:       store,
		spaceStore:            database.NewSpaceStore(),
		spaceResourceStore:    database.NewSpaceResourceStore(),
		runnerStatusCache:     make(map[string]types.StatusResponse),
		snowflakeNode:         node,
		eventPub:              &event.DefaultEventPublisher,
		runtimeFrameworkStore: database.NewRuntimeFrameworksStore(),
		deployConfig:          c,
		userStore:             database.NewUserStore(),
	}

	d.startJobs()
	return d, nil
}

func (d *deployer) checkOrderDetailByID(ctx context.Context, id int64) error {
	return nil
}

func (d *deployer) checkOrderDetail(ctx context.Context, dr types.DeployRepo) error {
	return nil
}

func (d *deployer) updateUserResourceByOrder(ctx context.Context, deploy *database.Deploy) error {
	return nil
}

func (d *deployer) releaseUserResourceByOrder(ctx context.Context, dr types.DeployRepo) error {
	return nil
}

func (d *deployer) startAccounting() {
	go d.startAcctFeeing()
	go d.startServiceConsuming()
}

func checkNodeResource(node types.NodeResourceInfo, hardware *types.HardWare) bool {
	if hardware.Gpu.Num != "" {
		gpu, err := strconv.Atoi(hardware.Gpu.Num)
		if err != nil {
			slog.Error("failed to parse hardware gpu ", slog.Any("error", err))
			return false
		}
		cpu, err := strconv.Atoi(hardware.Cpu.Num)
		if err != nil {
			slog.Error("failed to parse hardware cpu ", slog.Any("error", err))
			return false

		}
		if gpu <= int(node.AvailableXPU) && hardware.Gpu.Type == node.XPUModel && cpu <= int(node.AvailableCPU) {
			return true
		}
	} else {
		return true
	}
	return false
}

func (d *deployer) startJobs() {
	go func() {
		err := d.scheduler.Run()
		if err != nil {
			slog.Error("run scheduler failed", slog.Any("error", err))
		}
	}()
	go d.startAccounting()
}

func (d *deployer) getResources(ctx context.Context, clusterId string, clusterResponse *types.ClusterResponse) ([]types.NodeResourceInfo, error) {
	resources := make([]types.NodeResourceInfo, 0)
	for _, node := range clusterResponse.Nodes {
		resources = append(resources, node)
	}
	return resources, nil

}

func startAcctRequestFeeExtra(res types.StatusResponse) string {
	return ""
}

func updateDatabaseDeploy(dp *database.Deploy, dt types.DeployRepo) {
}

func updateEvaluationEnvHardware(env map[string]string, req types.EvaluationReq) {
	if req.Hardware.Gpu.Num != "" {
		env["GPU_NUM"] = req.Hardware.Gpu.Num
	}
}
