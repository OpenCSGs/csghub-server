//go:build saas || ee

package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"

	"github.com/bwmarrin/snowflake"
	"k8s.io/apimachinery/pkg/api/resource"
	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/reporter"
	"opencsg.com/csghub-server/component/reporter/sender"
)

type deployer struct {
	scheduler    scheduler.Scheduler
	imageBuilder imagebuilder.Builder
	imageRunner  imagerunner.Runner

	deployTaskStore       database.DeployTaskStore
	spaceStore            database.SpaceStore
	spaceResourceStore    database.SpaceResourceStore
	internalRootDomain    string
	snowflakeNode         *snowflake.Node
	eventPub              *event.EventPublisher
	runtimeFrameworkStore database.RuntimeFrameworksStore
	userResStore          database.UserResourcesStore
	deployConfig          common.DeployConfig
	userStore             database.UserStore
	clusterStore          database.ClusterInfoStore
	lokiClient            sender.LogSender
	logReporter           reporter.LogCollector
	acctClient            accounting.AccountingClient
}

func newDeployer(s scheduler.Scheduler, ib imagebuilder.Builder, ir imagerunner.Runner, c common.DeployConfig, logReporter reporter.LogCollector, cfg *config.Config) (*deployer, error) {
	store := database.NewDeployTaskStore()
	node, err := snowflake.NewNode(1)
	if err != nil || node == nil {
		return nil, fmt.Errorf("fail to generate uuid for inference service name error: %w", err)
	}
	acctClient, err := accounting.NewAccountingClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounting client: %w", err)
	}

	d := &deployer{
		scheduler:             s,
		imageBuilder:          ib,
		imageRunner:           ir,
		deployTaskStore:       store,
		spaceStore:            database.NewSpaceStore(),
		spaceResourceStore:    database.NewSpaceResourceStore(),
		snowflakeNode:         node,
		eventPub:              &event.DefaultEventPublisher,
		runtimeFrameworkStore: database.NewRuntimeFrameworksStore(),
		userResStore:          database.NewUserResourcesStore(),
		deployConfig:          c,
		userStore:             database.NewUserStore(),
		clusterStore:          database.NewClusterInfoStore(),
		lokiClient:            logReporter.GetSender(),
		logReporter:           logReporter,
		acctClient:            acctClient,
	}

	d.startJobs()
	return d, nil
}

func (d *deployer) checkOrderDetailByID(ctx context.Context, id int64) error {
	if id == 0 {
		return nil
	}
	ur, err := d.userResStore.FindUserResourcesByOrderDetailId(ctx, "", id)
	if err != nil {
		return fmt.Errorf("failed to check user resources by order detail id %d: %v", id, err)
	}
	if ur.DeployId != 0 {
		return fmt.Errorf("order detail id %d is already used", id)
	}
	return nil
}

func (d *deployer) checkOrderDetail(ctx context.Context, dr types.DeployRepo) error {
	if dr.OrderDetailID == 0 {
		return nil
	}
	ur, err := d.userResStore.FindUserResourcesByOrderDetailId(ctx, dr.UserUUID, dr.OrderDetailID)
	if err != nil {
		return fmt.Errorf("failed to check user resources by order detail id %d: %v", dr.OrderDetailID, err)
	}
	if ur.DeployId != 0 {
		return fmt.Errorf("order detail id %d is already used", dr.OrderDetailID)
	}
	return nil
}

func (d *deployer) updateUserResourceByOrder(ctx context.Context, deploy *database.Deploy) error {
	if deploy.OrderDetailID == 0 {
		return nil
	}
	ur, err := d.userResStore.FindUserResourcesByOrderDetailId(ctx, deploy.UserUUID, deploy.OrderDetailID)
	if err != nil {
		return fmt.Errorf("fail to find user resource, %w", err)
	}
	ur.DeployId = deploy.ID
	err = d.userResStore.UpdateDeployId(ctx, ur)
	if err != nil {
		return fmt.Errorf("fail to update user resource, %w", err)
	}
	return nil
}

func (d *deployer) releaseUserResourceByOrder(ctx context.Context, dr types.DeployRepo) error {
	if dr.OrderDetailID == 0 {
		return nil
	}
	ur, err := d.userResStore.FindUserResourcesByOrderDetailId(ctx, dr.UserUUID, dr.OrderDetailID)
	if err != nil {
		return fmt.Errorf("fail to find user resource, %w", err)
	}
	ur.DeployId = 0
	err = d.userResStore.UpdateDeployId(ctx, ur)
	if err != nil {
		return fmt.Errorf("fail to update user resource, %w", err)
	}
	return nil
}

func (d *deployer) startAccounting() {
	if d.deployConfig.ChargingEnable {
		d.registerStopDeployConsuming()
		d.startAcctOrderConsuming()
	}

	go d.startAcctMetering()

}

func checkNodeResource(node types.NodeResourceInfo, hardware *types.HardWare) bool {
	var xpuNumStr string
	var xpuType string
	xpuRequested := true

	if hardware.Cpu.Num != "" {
		requestedCPU, err := resource.ParseQuantity(hardware.Cpu.Num)
		if err != nil {
			slog.Error("failed to parse hardware cpu num", slog.String("cpu", hardware.Cpu.Num), slog.Any("error", err))
			return false
		}
		cores := float64(requestedCPU.MilliValue()) / 1000.0
		cpu := math.Round(cores*10) / 10
		if cpu > node.AvailableCPU {
			slog.Warn("insufficient cpu resources",
				slog.String("node name", node.NodeName),
				slog.Any("requested", cpu),
				slog.Any("available", node.AvailableCPU))
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
				slog.String("node name", node.NodeName),
				slog.Any("requestedGiB", requestedMemoryGiB),
				slog.Any("availableGiB", node.AvailableMem))
			return false
		}
	}

	switch {
	case hardware.Gpu.Num != "":
		xpuNumStr = hardware.Gpu.Num
		xpuType = hardware.Gpu.Type
	case hardware.Npu.Num != "":
		xpuNumStr = hardware.Npu.Num
		xpuType = hardware.Npu.Type
	case hardware.Gcu.Num != "":
		xpuNumStr = hardware.Gcu.Num
		xpuType = hardware.Gcu.Type
	case hardware.Mlu.Num != "":
		xpuNumStr = hardware.Mlu.Num
		xpuType = hardware.Mlu.Type
	case hardware.Dcu.Num != "":
		xpuNumStr = hardware.Dcu.Num
		xpuType = hardware.Dcu.Type
	case hardware.GPGpu.Num != "":
		xpuNumStr = hardware.GPGpu.Num
		xpuType = hardware.GPGpu.Type
	default:
		xpuRequested = false
	}

	if xpuRequested {
		xpu, err := strconv.Atoi(xpuNumStr)
		if err != nil {
			slog.Error("failed to parse hardware xpu num", slog.Any("error", err))
			return false
		}
		if xpu > int(node.AvailableXPU) || xpuType != node.XPUModel {
			slog.Warn("insufficient xpu resources",
				slog.String("node name", node.NodeName),
				slog.Any("requested xpu type", xpuType),
				slog.Any("xpu model of node", node.XPUModel),
				slog.Any("requestedXPU", xpu),
				slog.Any("availableXPU", node.AvailableXPU))
			return false
		}
	}

	return true
}

func (d *deployer) getResources(ctx context.Context, clusterId string, clusterResponse *types.ClusterResponse) ([]types.NodeResourceInfo, error) {
	// get reserved resources
	userResources, err := d.userResStore.GetReservedUserResources(ctx, "", clusterId)
	if err != nil {
		return nil, err
	}
	resources := make([]types.NodeResourceInfo, 0)
	for _, node := range clusterResponse.Nodes {
		resources = append(resources, node)
	}
	for _, r := range userResources {
		for i := range resources {
			if resources[i].AvailableXPU >= int64(r.XPUNum) {
				resources[i].AvailableXPU -= int64(r.XPUNum)
				resources[i].ReservedXPU += int64(r.XPUNum)
			}
		}
	}
	return resources, nil

}

func startAcctRequestFeeExtra(deploy database.Deploy, source string) string {
	extraStr := ""
	if deploy.OrderDetailID != 0 {
		extraStr = fmt.Sprintf("\"%s\": \"%d\"", types.OrderDetailID, deploy.OrderDetailID)
	}
	if len(source) > 0 {
		if len(extraStr) > 0 {
			extraStr += ", "
		}
		extraStr += fmt.Sprintf("\"%s\": \"%s\"", types.MeterFromSource, source)
	}
	if len(extraStr) > 0 {
		extraStr = "{ " + extraStr + " }"
	}
	return extraStr
}

func updateDatabaseDeploy(dp *database.Deploy, dt types.DeployRepo) {
	dp.OrderDetailID = dt.OrderDetailID
}
