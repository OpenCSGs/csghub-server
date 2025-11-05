//go:build !ee && !saas

package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"

	"opencsg.com/csghub-server/component/reporter"

	"github.com/bwmarrin/snowflake"
	"k8s.io/apimachinery/pkg/api/resource"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/reporter/sender"
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
	deployConfig          common.DeployConfig
	userStore             database.UserStore
	clusterStore          database.ClusterInfoStore
	lokiClient            sender.LogSender
	logReporter           reporter.LogCollector
}

func newDeployer(s scheduler.Scheduler, ib imagebuilder.Builder, ir imagerunner.Runner, c common.DeployConfig, logReporter reporter.LogCollector, cfg *config.Config, startJobs bool) (*deployer, error) {

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
		clusterStore:          database.NewClusterInfoStore(),
		lokiClient:            logReporter.GetSender(),
		logReporter:           logReporter,
	}

	if startJobs {
		d.startJobs()
	}
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
	go d.startAcctMetering()
}

func checkNodeResource(node types.NodeResourceInfo, hardware *types.HardWare) bool {

	if hardware.Cpu.Num != "" {
		requestedCPU, err := resource.ParseQuantity(hardware.Cpu.Num)
		if err != nil {
			slog.Error("failed to parse hardware cpu num", slog.String("cpu", hardware.Cpu.Num), slog.Any("error", err))
			return false
		}
		cores := float64(requestedCPU.MilliValue()) / 1000.0
		cpu := math.Round(cores*10) / 10
		if cpu > node.AvailableCPU {
			return false
		}
	}

	if hardware.Memory != "" {
		// use resource.ParseQuantity parse "2Gi", "1024M" ...
		requestedMemory, err := resource.ParseQuantity(hardware.Memory)
		if err != nil {
			slog.Error("failed to parse hardware memory", slog.String("memory", hardware.Memory), slog.Any("error", err))
			return false
		}

		requestedMemoryGiB := float32(requestedMemory.Value()) / (1024 * 1024 * 1024)

		if requestedMemoryGiB > node.AvailableMem {
			slog.Warn("insufficient memory resources",
				slog.Any("requestedGiB", requestedMemoryGiB),
				slog.Any("availableGiB", node.AvailableMem))
			return false
		}
	}

	if hardware.Gpu.Num != "" {
		gpu, err := strconv.Atoi(hardware.Gpu.Num)
		if err != nil {
			slog.Error("failed to parse hardware gpu ", slog.Any("error", err))
			return false
		}
		if gpu > int(node.AvailableXPU) || hardware.Gpu.Type != node.XPUModel {
			return false
		}
	}

	return true
}

func (d *deployer) getResources(ctx context.Context, clusterId string, clusterResponse *types.ClusterResponse) ([]types.NodeResourceInfo, error) {
	resources := make([]types.NodeResourceInfo, 0)
	for _, node := range clusterResponse.Nodes {
		resources = append(resources, node)
	}
	return resources, nil
}

func startAcctRequestFeeExtra(deploy database.Deploy, source string) string {
	if len(source) > 0 {
		return fmt.Sprintf("{ \"%s\": \"%s\" }", types.MeterFromSource, source)
	}
	return ""
}

func updateDatabaseDeploy(dp *database.Deploy, dt types.DeployRepo) {
}
