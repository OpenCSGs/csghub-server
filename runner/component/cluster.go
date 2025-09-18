package component

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	rcommon "opencsg.com/csghub-server/runner/common"
)

type clusterComponentImpl struct {
	k8sNameSpace            string
	env                     *config.Config
	spaceDockerRegBase      string
	modelDockerRegBase      string
	imagePullSecret         string
	informerSyncPeriodInMin int
	clusterStore            database.ClusterInfoStore
	clusterPool             *cluster.ClusterPool
}

type ClusterComponent interface {
	ByClusterID(ctx context.Context, clusterId string) (clusterInfo database.ClusterInfo, err error)
	GetResourceByID(ctx context.Context, clusterId string) (types.ResourceStatus, map[string]types.NodeResourceInfo, error)
}

func NewClusterComponent(config *config.Config, clusterPool *cluster.ClusterPool) ClusterComponent {
	sc := &clusterComponentImpl{
		k8sNameSpace:            config.Cluster.SpaceNamespace,
		env:                     config,
		spaceDockerRegBase:      config.Space.DockerRegBase,
		modelDockerRegBase:      config.Model.DockerRegBase,
		imagePullSecret:         config.Space.ImagePullSecret,
		informerSyncPeriodInMin: config.Space.InformerSyncPeriodInMin,
		clusterStore:            database.NewClusterInfoStore(),
		clusterPool:             clusterPool,
	}
	go sc.initCluster()
	go sc.heartBeat()
	return sc
}

// InitCluster init cluster
func (s *clusterComponentImpl) initCluster() {
	// send cluster event
	for _, c := range s.clusterPool.Clusters {
		if c.ConnectMode == types.ConnectModeInCluster {
			go func(c *cluster.Cluster) {
				data := types.ClusterEvent{
					ClusterID:     c.ID,
					ClusterConfig: types.DefaultClusterCongfig,
					Region:        c.Region,
					Mode:          c.ConnectMode,
				}
				event := &types.WebHookSendEvent{
					WebHookHeader: types.WebHookHeader{
						EventType: types.RunnerClusterCreate,
						EventTime: time.Now().Unix(),
						ClusterID: c.ID,
						DataType:  types.WebHookDataTypeObject,
					},
					Data: data,
				}
				err := rcommon.Push(s.env.Runner.WebHookEndpoint, s.env.APIToken, event)
				if err != nil {
					slog.Error("failed to push cluster status event during start runner", slog.Any("error", err), slog.Any("event", event))
				}
			}(c)
		}

		// watch cluster configmap change
		go func(c *cluster.Cluster) {
			watcher := &clusterWatcher{
				cluster: c,
				env:     s.env,
			}
			configmapWatch, err := rcommon.NewConfigmapWatcher(
				watcher.cluster.Client,
				watcher,
				s.env)
			if err != nil {
				slog.Error("failed to create configmap watcher", slog.String("cluster", c.CID), slog.Any("error", err))
				return
			}
			slog.Info("start watching configmap", slog.String("cluster", c.CID), slog.String("namespace", s.env.Cluster.SpaceNamespace), slog.String("configmap", s.env.Runner.WatchConfigmapName))
			configmapWatch.Watch(context.Background())
		}(c)
	}
}

func (c *clusterComponentImpl) ByClusterID(ctx context.Context, clusterId string) (clusterInfo database.ClusterInfo, err error) {
	return c.clusterStore.ByClusterID(ctx, clusterId)
}

func (c *clusterComponentImpl) GetResourceByID(ctx context.Context, clusterId string) (types.ResourceStatus, map[string]types.NodeResourceInfo, error) {
	client, err := c.clusterPool.GetClusterByID(ctx, clusterId)
	if err != nil {
		return "", nil, fmt.Errorf("failed to find cluster, error: %w", err)
	}
	return client.GetResourceAvailability(c.env)
}

func (c *clusterComponentImpl) heartBeat() {
	for {
		startTime := time.Now().Unix()
		if len(c.clusterPool.Clusters) > 0 {
			c.pushHeartBeatEvent()
		}
		escapedTime := time.Now().Unix() - startTime
		sleepTime := int64(c.env.Runner.HearBeatIntervalInSec) - escapedTime
		sleepTime = int64(math.Max(1, float64(sleepTime)))
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
}

func (c *clusterComponentImpl) pushHeartBeatEvent() {
	eventData := &types.HearBeatEvent{
		Running:     []string{},
		Unavailable: []string{},
	}
	for _, cluster := range c.clusterPool.Clusters {
		err := c.checkIfClusterAvailable(cluster)
		if err != nil {
			slog.Warn("failed to check if cluster is available", slog.Any("error", err), slog.Any("cluster", cluster))
			eventData.Unavailable = append(eventData.Unavailable, cluster.ID)
		} else {
			eventData.Running = append(eventData.Running, cluster.ID)
		}

	}
	event := &types.WebHookSendEvent{
		WebHookHeader: types.WebHookHeader{
			EventType: types.RunnerHeartbeat,
			EventTime: time.Now().Unix(),
			DataType:  types.WebHookDataTypeArray,
		},
		Data: eventData,
	}
	err := rcommon.Push(c.env.Runner.WebHookEndpoint, c.env.APIToken, event)
	if err != nil {
		slog.Error("failed to report cluster heartbeat event", slog.Any("error", err), slog.Any("event", event),
			slog.Any("HearBeatIntervalInSec", c.env.Runner.HearBeatIntervalInSec))
	} else {
		go rcommon.PushCachedFailedEvents(c.env.Runner.WebHookEndpoint, c.env.APIToken)
	}
}

func (c *clusterComponentImpl) checkIfClusterAvailable(cluster *cluster.Cluster) error {
	_, err := cluster.Client.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("cluster %s is unavailable: %w", cluster.ID, err)
	}
	return nil
}
