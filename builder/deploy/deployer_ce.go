//go:build !ee && !saas

package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"

	"opencsg.com/csghub-server/component/reporter"
	runnerTypes "opencsg.com/csghub-server/runner/types"

	"github.com/bwmarrin/snowflake"
	"k8s.io/apimachinery/pkg/api/resource"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/reporter/sender"
)

type deployer struct {
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
	argoWorkflowStore     database.ArgoWorkFlowStore
	deployConfig          common.DeployConfig
	userStore             database.UserStore
	clusterStore          database.ClusterInfoStore
	lokiClient            sender.LogSender
	logReporter           reporter.LogCollector
	config                *config.Config
}

func newDeployer(ib imagebuilder.Builder, ir imagerunner.Runner, c common.DeployConfig, logReporter reporter.LogCollector, config *config.Config, startJobs bool) (*deployer, error) {

	store := database.NewDeployTaskStore()
	node, err := snowflake.NewNode(1)
	if err != nil || node == nil {
		slog.Error("fail to generate uuid for inference service name", slog.Any("error", err))
		return nil, err
	}

	d := &deployer{
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
		lokiClient:            safeGetSender(logReporter),
		logReporter:           logReporter,
		argoWorkflowStore:     database.NewArgoWorkFlowStore(),
		config:                config,
	}
	if startJobs {
		d.startJobs()
	}
	return d, nil
}

func (d *deployer) checkOrderDetailByID(ctx context.Context, id int64) error {
	return nil
}

func (d *deployer) checkOrderDetail(ctx context.Context, dr types.DeployRequest) error {
	return nil
}

func (d *deployer) updateUserResourceByOrder(ctx context.Context, deploy *database.Deploy) error {
	return nil
}

func (d *deployer) releaseUserResourceByOrder(ctx context.Context, dr types.DeployRequest) error {
	return nil
}

func (d *deployer) startAccounting() {
	go d.startAcctMetering()
}

func checkNodeResource(node types.NodeResourceInfo, hardware *types.HardWare,
	config *config.Config, VXPUConfig map[string]string) types.ResourceAvailableStatus {
	if node.NodeStatus == string(types.NodeStatusOffline) {
		return types.ResourceAvailableStatus{
			Available: false,
			NodeName:  node.NodeName,
			Reason:    types.UnAvailableTypeNodeOffline,
		}
	}

	if !config.Cluster.AllowCPUResScheduleToGPUNode && isCPUOnlyWorkload(hardware) && isXPUNode(node) {
		return types.ResourceAvailableStatus{
			Available: false,
			NodeName:  node.NodeName,
			Reason:    types.UnAvailableTypeDisableScheduling,
		}
	}

	if hardware.Cpu.Num != "" {
		requestedCPU, err := resource.ParseQuantity(hardware.Cpu.Num)
		if err != nil {
			slog.Error("check node resource - failed to parse hardware cpu num",
				slog.String("cpu", hardware.Cpu.Num),
				slog.Any("error", err),
				slog.Any("NodeName", node.NodeName))
			return types.ResourceAvailableStatus{
				Available: false,
				NodeName:  node.NodeName,
				Reason:    types.UnAvailableTypeInvalidCPUNum,
			}
		}
		cores := float64(requestedCPU.MilliValue()) / 1000.0
		cpu := math.Round(cores*10) / 10
		if cpu > node.AvailableCPU {
			return types.ResourceAvailableStatus{
				Available: false,
				NodeName:  node.NodeName,
				Reason:    types.UnAvailableTypeInsufficientCPU,
			}
		}
	}

	if hardware.Memory != "" {
		// use resource.ParseQuantity parse "2Gi", "1024M" ...
		requestedMemory, err := resource.ParseQuantity(hardware.Memory)
		if err != nil {
			slog.Error("check node resource - failed to parse hardware memory",
				slog.String("memory", hardware.Memory),
				slog.Any("error", err),
				slog.Any("NodeName", node.NodeName))
			return types.ResourceAvailableStatus{
				Available: false,
				NodeName:  node.NodeName,
				Reason:    types.UnAvailableTypeInvalidMemorySize,
			}
		}

		requestedMemoryGiB := float32(requestedMemory.Value()) / (1024 * 1024 * 1024)

		if requestedMemoryGiB > node.AvailableMem {
			slog.Warn("check node resource - insufficient memory resources",
				slog.Any("requestedGiB", requestedMemoryGiB),
				slog.Any("availableGiB", node.AvailableMem),
				slog.Any("NodeName", node.NodeName))
			return types.ResourceAvailableStatus{
				Available: false,
				NodeName:  node.NodeName,
				Reason:    types.UnAvailableTypeInsufficientMemory,
			}
		}
	}

	if hardware.Gpu.Num != "" {
		if hardware.Gpu.Type != node.XPUModel {
			slog.Warn("check node resource - incorrect node xpu type",
				slog.String("node name", node.NodeName),
				slog.Any("requested xpu type", hardware.Gpu.Type),
				slog.Any("xpu model of node", node.XPUModel),
				slog.Any("hardware", hardware))
			return types.ResourceAvailableStatus{
				Available: false,
				NodeName:  node.NodeName,
				Reason:    types.UnAvailableTypeInvalidXPUType,
			}
		}

		gpu, err := strconv.Atoi(hardware.Gpu.Num)
		if err != nil {
			slog.Error("check node resource - failed to parse hardware gpu num",
				slog.String("gpu", hardware.Gpu.Num),
				slog.Any("error", err),
				slog.Any("NodeName", node.NodeName))
			return types.ResourceAvailableStatus{
				Available: false,
				NodeName:  node.NodeName,
				Reason:    types.UnAvailableTypeInvalidXPUNum,
			}
		}
		if gpu > int(node.AvailableXPU) {
			return types.ResourceAvailableStatus{
				Available: false,
				NodeName:  node.NodeName,
				Reason:    types.UnAvailableTypeInsufficientXPU,
			}
		}
	}

	return types.ResourceAvailableStatus{
		Available: true,
		NodeName:  node.NodeName,
		Reason:    types.AvailableTypeOK,
	}
}

func (d *deployer) LabelNode(ctx context.Context, req *types.NodeLabel) error {
	return nil
}

func (d *deployer) calcResources(ctx context.Context, clusterId string, clusterResponse *types.ClusterRes) ([]types.NodeResourceInfo, error) {
	return clusterResponse.Resources, nil
}

func startAcctRequestFeeExtra(deploy database.Deploy, source string) string {
	if len(source) > 0 {
		return fmt.Sprintf("{ \"%s\": \"%s\" }", types.MeterFromSource, source)
	}
	return ""
}

func updateDatabaseDeploy(dp *database.Deploy, dt types.DeployRequest) {
}

func (d *deployer) DeleteSandbox(ctx context.Context, req *runnerTypes.SandboxDeleteRequest) error {
	return nil
}
