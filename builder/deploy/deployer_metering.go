package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/redis"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (d *deployer) startAcctMetering() {
	for {
		// run once every STARHUB_SERVER_SYNC_IN_MINUTES minutes
		// accounting interval in min, get from env config
		startTimeSec := time.Now().Unix()
		errLock := d.deployConfig.RedisLocker.GetServerAcctMeteringLock()
		if errLock != nil && errors.Is(errLock, redis.ErrLockAcquire) {
			slog.Warn("skip deployer metering only for fail getting distriubte lock", slog.Any("error", errLock))
			d.sleepAcctInterval(startTimeSec)
			continue
		}
		d.startAcctMeteringProcess()
		d.sleepAcctInterval(startTimeSec)
		if errLock != nil {
			slog.Warn("do metering with get distributed lock error", slog.Any("error", errLock))
		} else {
			ok, err := d.deployConfig.RedisLocker.ReleaseServerAcctMeteringLock()
			if err != nil {
				slog.Error("failed to release deployer metering lock", slog.Any("error", err), slog.Any("ok", ok))
			}
		}
	}
}

func (d *deployer) sleepAcctInterval(startTimeSec int64) {
	// sleep remain seconds until next metering interval
	totalSecond := int64(d.eventPub.SyncInterval * 60)
	endTimeSec := time.Now().Unix()
	remainSeconds := totalSecond - (endTimeSec - startTimeSec)
	slog.Debug("sleep until next metering interval for deployer metering", slog.Any("remain seconds", remainSeconds))
	if remainSeconds > 0 {
		time.Sleep(time.Duration(remainSeconds) * time.Second)
	}
}

func (d *deployer) getResourceMap() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resList, err := d.spaceResourceStore.FindAll(ctx)
	resources := make(map[string]string)
	if err != nil {
		slog.Error("failed to get hub resource", slog.Any("error", err))
	} else {
		for _, res := range resList {
			resources[strconv.FormatInt(res.ID, 10)] = res.Name
		}
	}
	return resources
}

func (d *deployer) getClusterMap() map[string]database.ClusterInfo {
	clusterMap := make(map[string]database.ClusterInfo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clusters, err := d.clusterStore.List(ctx)
	if err != nil {
		slog.Error("failed to get cluster list", slog.Any("error", err))
		return clusterMap
	}

	for _, cluster := range clusters {
		clusterMap[cluster.ClusterID] = cluster
	}
	return clusterMap
}

func (d *deployer) startAcctMeteringProcess() {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	runningDeploys, err := d.deployTaskStore.ListAllRunningDeploys(ctxTimeout)
	if err != nil {
		slog.Error("skip meterting due to fail to get all running deploys in deployer", slog.Any("error", err))
		return
	}
	resMap := d.getResourceMap()
	slog.Debug("get resources map", slog.Any("resMap", resMap))
	eventTime := time.Now()
	clusterMap := d.getClusterMap()
	for _, deploy := range runningDeploys {
		d.startAcctMeteringRequest(ctxTimeout, resMap, clusterMap, deploy, eventTime)
	}

	runningEvaluations, err := d.argoWorkflowStore.ListAllRunningEvaluations(ctxTimeout)
	if err != nil {
		slog.Error("skip meterting due to fail to get all running evaluations in deployer", slog.Any("error", err))
		return
	}
	for _, evaluation := range runningEvaluations {
		d.startAcctForEvaluations(ctxTimeout, clusterMap, evaluation)
	}
}

func (d *deployer) startAcctMeteringRequest(ctx context.Context, resMap map[string]string, clusterMap map[string]database.ClusterInfo,
	deploy database.Deploy, eventTime time.Time) {
	// skip for cluster does not exist
	cluster, ok := clusterMap[deploy.ClusterID]
	if !ok {
		slog.WarnContext(ctx, "skip metering for no valid cluster found by id",
			slog.Any("deploy_id", deploy.ID), slog.Any("svc_name", deploy.SvcName),
			slog.Any("cluster_id", deploy.ClusterID))
		return
	}

	// skip metering for cluster is not running
	if cluster.Status != types.ClusterStatusRunning {
		slog.WarnContext(ctx, "skip metering for cluster is not running",
			slog.Any("deploy_id", deploy.ID), slog.Any("svc_name", deploy.SvcName),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region),
			slog.Any("zone", cluster.Zone), slog.Any("provider", cluster.Provider))
		return
	}

	// cluster does not have hearbeat more than 10 mins, ignore it
	if time.Now().Unix()-cluster.UpdatedAt.Unix() > int64(d.deployConfig.HeartBeatTimeInSec*2) {
		slog.WarnContext(ctx, fmt.Sprintf("skip metering for cluster does not have heartbeat more than %d seconds",
			(d.deployConfig.HeartBeatTimeInSec*2)),
			slog.Any("deploy_id", deploy.ID), slog.Any("svc_name", deploy.SvcName),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region),
			slog.Any("zone", cluster.Zone), slog.Any("provider", cluster.Provider),
			slog.Any("updated_at", cluster.UpdatedAt))
		return
	}

	// ignore deploy without sku resource
	if len(deploy.SKU) < 1 {
		return
	}
	resName, exists := resMap[deploy.SKU]
	if !exists {
		slog.WarnContext(ctx, "Did not find space resource by id for metering",
			slog.Any("id", deploy.SKU), slog.Any("deploy_id", deploy.ID), slog.Any("svc_name", deploy.SvcName))
		return
	}
	slog.DebugContext(ctx, "metering deploy", slog.Any("deploy", deploy))
	sceneType := common.GetValidSceneType(deploy.Type)
	if sceneType == types.SceneUnknow {
		slog.ErrorContext(ctx, "invalid deploy type of service for metering", slog.Any("deploy", deploy))
		return
	}
	if sceneType == types.SceneModelServerless {
		// skip metering for model serverless scene
		return
	}

	var hardware types.HardWare
	if err := json.Unmarshal([]byte(deploy.Hardware), &hardware); err != nil {
		slog.ErrorContext(ctx, "Deploy hardware is invalid format", "hardware", deploy.Hardware, "deploy_id", deploy.ID)
		return
	}

	// Get replica count and multiply value by instance count
	replicaCount := 1
	// only check for single node deploy, muti-node don't support replica
	// and deploy request replica count is greater than 1
	if hardware.Replicas < 2 && (deploy.MaxReplica > 1 || deploy.MinReplica > 1) {
		runningReplicaCount := 0
		for _, inst := range deploy.Instances {
			if inst.Status == string(types.ClusterStatusRunning) {
				runningReplicaCount++
			}
		}
		if runningReplicaCount > replicaCount {
			replicaCount = runningReplicaCount
		}
	}

	extra := startAcctRequestFeeExtra(deploy, d.deployConfig.UniqueServiceName)
	event := types.MeteringEvent{
		Uuid:         uuid.New(), //v4
		UserUUID:     deploy.UserUUID,
		Value:        int64(d.eventPub.SyncInterval) * int64(replicaCount),
		ValueType:    types.TimeDurationMinType,
		Scene:        int(sceneType),
		OpUID:        "",
		ResourceID:   deploy.SKU,
		ResourceName: resName,
		CustomerID:   deploy.SvcName,
		CreatedAt:    eventTime,
		Extra:        extra,
	}
	str, err := json.Marshal(event)
	if err != nil {
		slog.ErrorContext(ctx, "error marshal metering event", slog.Any("event", event), slog.Any("error", err))
		return
	}
	err = d.eventPub.PublishMeteringEvent(str)
	if err != nil {
		slog.ErrorContext(ctx, "failed to pub metering event", slog.Any("data", string(str)), slog.Any("error", err))
	} else {
		slog.DebugContext(ctx, "pub metering event success", slog.Any("data", string(str)))
	}
}

func (d *deployer) startAcctForEvaluations(ctx context.Context, clusterMap map[string]database.ClusterInfo,
	evaluation database.ArgoWorkflow) {
	// skip for cluster does not exist
	cluster, ok := clusterMap[evaluation.ClusterID]
	if !ok {
		slog.WarnContext(ctx, "skip evaluation metering for no valid cluster found by id",
			slog.Any("task_id", evaluation.TaskId),
			slog.Any("cluster_id", evaluation.ClusterID))
		return
	}

	// skip metering for cluster is not running
	if cluster.Status != types.ClusterStatusRunning {
		slog.WarnContext(ctx, "skip evaluation metering for cluster status is not running",
			slog.Any("task_id", evaluation.TaskId),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region))
		return
	}

	// cluster does not have hearbeat more than 10 mins, ignore it
	if time.Now().Unix()-cluster.UpdatedAt.Unix() > int64(d.deployConfig.HeartBeatTimeInSec*2) {
		slog.WarnContext(ctx, fmt.Sprintf("skip evaluation metering for cluster does not have heartbeat more than %d seconds",
			(d.deployConfig.HeartBeatTimeInSec*2)),
			slog.Any("task_id", evaluation.TaskId),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region),
			slog.Any("zone", cluster.Zone), slog.Any("provider", cluster.Provider),
			slog.Any("updated_at", cluster.UpdatedAt))
		return
	}

	// ignore deploy without sku resource
	if evaluation.ResourceId < 1 {
		slog.WarnContext(ctx, "skip evaluation metering for without sku resource",
			slog.Any("task_id", evaluation.TaskId),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region))
		return
	}

	event := types.MeteringEvent{
		Uuid:         uuid.New(),
		UserUUID:     evaluation.UserUUID,
		Value:        int64(d.eventPub.SyncInterval),
		ValueType:    types.TimeDurationMinType,
		Scene:        int(types.SceneEvaluation),
		OpUID:        "",
		ResourceID:   strconv.FormatInt(evaluation.ResourceId, 10),
		ResourceName: evaluation.ResourceName,
		CustomerID:   evaluation.TaskId,
		CreatedAt:    time.Now(),
	}
	str, err := json.Marshal(event)
	if err != nil {
		slog.ErrorContext(ctx, "error marshal evaluation metering event", slog.Any("event", event), slog.Any("error", err))
		return
	}
	err = d.eventPub.PublishMeteringEvent(str)
	if err != nil {
		slog.ErrorContext(ctx, "failed to pub evaluation metering event", slog.Any("data", string(str)), slog.Any("error", err))
	} else {
		slog.DebugContext(ctx, "pub metering evaluation event success", slog.Any("data", string(str)))
	}
}
