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
	env          *config.Config
	clusterStore database.ClusterInfoStore
	clusterPool  cluster.Pool
}

type ClusterComponent interface {
	ByClusterID(ctx context.Context, clusterId string) (clusterInfo database.ClusterInfo, err error)
	GetResourceByID(ctx context.Context, clusterId string) (types.ResourceStatus, map[string]types.NodeResourceInfo, error)
}

func NewClusterComponent(config *config.Config, clusterPool cluster.Pool) ClusterComponent {
	sc := &clusterComponentImpl{
		env:          config,
		clusterStore: database.NewClusterInfoStore(),
		clusterPool:  clusterPool,
	}
	go sc.initCluster()
	go sc.heartBeat()
	return sc
}

// InitCluster init cluster
func (s *clusterComponentImpl) initCluster() {
	// send cluster event
	clusters := s.clusterPool.GetAllCluster()
	for _, c := range clusters {
		if c.ConnectMode == types.ConnectModeInCluster {
			go func(c *cluster.Cluster) {
				data := types.ClusterEvent{
					ClusterID:        c.ID,
					ClusterConfig:    types.DefaultClusterCongfig,
					Region:           c.Region,
					StorageClass:     c.StorageClass,
					Mode:             c.ConnectMode,
					NetworkInterface: c.NetworkInterface,
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
					slog.Error("failed to push cluster create event during start runner", slog.Any("error", err), slog.Any("event", event))
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
			slog.Info("start watching configmap",
				slog.String("cluster", c.CID), slog.Any("cluster_mode", c.ConnectMode),
				slog.String("namespace", s.env.Runner.RunnerNamespace),
				slog.String("configmap_name", s.env.Runner.WatchConfigmapName))
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
		clusters := c.clusterPool.GetAllCluster()
		if len(clusters) > 0 {
			c.pushHeartBeatEvent()
		}
		escapedTime := time.Now().Unix() - startTime
		sleepTime := int64(c.env.Runner.HearBeatIntervalInSec) - escapedTime
		sleepTime = int64(math.Max(1, float64(sleepTime)))
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
}

func (c *clusterComponentImpl) pushHeartBeatEvent() {
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(c.env.Runner.HearBeatIntervalInSec)*time.Second)
	defer cancel()

	clusterResArray := c.collectAllClusters(ctx)

	event := &types.WebHookSendEvent{
		WebHookHeader: types.WebHookHeader{
			EventType: types.RunnerHeartbeat,
			EventTime: time.Now().Unix(),
			DataType:  types.WebHookDataTypeArray,
		},
		Data: clusterResArray,
	}

	err := rcommon.Push(c.env.Runner.WebHookEndpoint, c.env.APIToken, event)
	slog.InfoContext(ctx, "push cluster heart beat event", slog.Any("len(clusters)", len(clusterResArray)))
	if err != nil {
		slog.Error("failed to report cluster heartbeat resource event",
			slog.Any("error", err),
			slog.Any("event", event),
			slog.Any("HearBeatIntervalInSec", c.env.Runner.HearBeatIntervalInSec))
	} else {
		go rcommon.PushCachedFailedEvents(c.env.Runner.WebHookEndpoint, c.env.APIToken)
	}
	slog.DebugContext(ctx, "heartbeat_event_sent", slog.Any("event_body", event.Data))
}

func (c *clusterComponentImpl) collectAllClusters(ctx context.Context) []*types.ClusterRes {
	clusters := c.clusterPool.GetAllCluster()
	clusterResArray := []*types.ClusterRes{}
	for _, cluster := range clusters {
		clusterInfo, err := c.collectResourceByID(ctx, cluster.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to collect cluster resource by clusterId %s, error: %w", cluster.ID, err)
			continue
		}
		clusterResArray = append(clusterResArray, clusterInfo)
	}
	return clusterResArray
}

func (c *clusterComponentImpl) collectResourceByID(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	cInfo, err := c.ByClusterID(ctx, clusterId)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster by clusterId %s, error: %w", clusterId, err)
	}
	clusterInfo := types.ClusterRes{}
	clusterInfo.Region = cInfo.Region
	clusterInfo.Zone = cInfo.Zone
	clusterInfo.Provider = cInfo.Provider
	clusterInfo.ClusterID = cInfo.ClusterID
	clusterInfo.StorageClass = cInfo.StorageClass
	clusterInfo.Enable = cInfo.Enable
	availabilityStatus, resourceAvaliable, err := c.GetResourceByID(ctx, clusterId)
	if err != nil {
		return nil, fmt.Errorf("failed to collect cluster physical resource by clusterId %s, error: %w", clusterId, err)
	}
	for _, v := range resourceAvaliable {
		clusterInfo.Resources = append(clusterInfo.Resources, v)
	}
	clusterInfo.ResourceStatus = availabilityStatus
	return &clusterInfo, nil
}
