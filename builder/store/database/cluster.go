package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type clusterInfoStoreImpl struct {
	db *DB
}

type ClusterInfoStore interface {
	Add(ctx context.Context, clusterConfig string, region string, mode types.ClusterMode) (*ClusterInfo, error)
	AddByClusterID(ctx context.Context, clusterId string, region string) (*ClusterInfo, error)
	Update(ctx context.Context, clusterInfo ClusterInfo) error
	UpdateByClusterID(ctx context.Context, cluster types.ClusterEvent) error
	ByClusterID(ctx context.Context, clusterId string) (clusterInfo ClusterInfo, err error)
	ByClusterConfig(ctx context.Context, clusterConfig string) (clusterInfo ClusterInfo, err error)
	List(ctx context.Context) ([]ClusterInfo, error)
	BatchUpdateStatus(ctx context.Context, statusEvent []*types.ClusterRes) error
	GetClusterResources(ctx context.Context, clusterID string) (*types.ClusterRes, error)
	FindNodeByClusterID(ctx context.Context, clusterID string) ([]ClusterNode, error)
	ListAllNodes(ctx context.Context) ([]ClusterNodeWithRegion, error)
	GetNodeByID(ctx context.Context, id int64) (*ClusterNodeWithRegion, error)
	UpdateNode(ctx context.Context, id int64, enableVXPU bool) (*ClusterNode, error)
	GetClusterNodeByID(ctx context.Context, id int64) (*ClusterNode, error)
	UpdateClusterNodeByNode(ctx context.Context, node ClusterNode) error
	AddNodeOwnership(ctx context.Context, ownership ClusterNodeOwnership) error
	DeleteNodeOwnership(ctx context.Context, clusterNodeID int64) error
	GetNodeOwnership(ctx context.Context, clusterNodeID int64) (*ClusterNodeOwnership, error)
	UpdateNodeOwnership(ctx context.Context, ownership ClusterNodeOwnership) error
	ExecuteInTx(ctx context.Context, fn func(ctx context.Context, store ClusterInfoStore) error) error
}

func NewClusterInfoStore() ClusterInfoStore {
	return &clusterInfoStoreImpl{
		db: defaultDB,
	}
}

func NewClusterInfoStoreWithDB(db *DB) ClusterInfoStore {
	return &clusterInfoStoreImpl{
		db: db,
	}
}

type ClusterInfo struct {
	ClusterID        string               `bun:",pk" json:"cluster_id"`
	ClusterConfig    string               `bun:",notnull" json:"cluster_config"`
	StorageClass     string               `bun:"," json:"storage_class"`
	Region           string               `bun:"," json:"region"`
	Zone             string               `bun:"," json:"zone"`     //cn-beijing
	Provider         string               `bun:"," json:"provider"` //ali
	Enable           bool                 `bun:",notnull" json:"enable"`
	Status           types.ClusterStatus  `bun:"," json:"status"`                  //running, unavailable
	RunnerEndpoint   string               `bun:"endpoint," json:"runner_endpoint"` //runner in k8s api endpoint
	NetworkInterface string               `bun:"," json:"network_interface"`       //used for multi-host, e.g., eth0
	Mode             types.ClusterMode    `bun:"," json:"mode"`                    //used for multi-host, e.g., host, bridge
	AppEndpoint      string               `bun:"," json:"app_endpoint"`            //runner app endpoint
	ResourceStatus   types.ResourceStatus `bun:"," json:"resource_status"`
	times
}

type ClusterNode struct {
	ID          int64               `bun:",pk,autoincrement" json:"id"`
	ClusterID   string              `bun:",notnull" json:"cluster_id"`
	Name        string              `bun:",notnull" json:"name"`
	Status      string              `bun:",nullzero" json:"status"`
	Labels      map[string]string   `bun:",type:jsonb,nullzero" json:"labels"`
	EnableVXPU  bool                `bun:",default:false" json:"enable_vxpu"`
	ComputeCard string              `bun:",nullzero" json:"compute_card"`
	Hardware    types.NodeHardware  `bun:",type:jsonb,nullzero" json:"hardware"`
	Processes   []types.ProcessInfo `bun:",type:jsonb,nullzero" json:"processes"`
	Exclusive   bool                `bun:",default:false" json:"exclusive"`
	times
}

type ClusterNodeOwnership struct {
	ID            int64  `bun:",pk,autoincrement" json:"id"`
	ClusterNodeID int64  `bun:",notnull" json:"cluster_node_id"`
	ClusterID     string `bun:",notnull" json:"cluster_id"`
	UserUUID      string `bun:",nullzero" json:"user_uuid"`
	OrgUUID       string `bun:",nullzero" json:"org_uuid"`
	times
}

type ClusterNodeWithRegion struct {
	ClusterNode
	ClusterRegion  string `json:"cluster_region"`
	TaskRunningNum int    `json:"task_running_num"`
}

func (r *clusterInfoStoreImpl) Add(ctx context.Context, clusterConfig string, region string, mode types.ClusterMode) (*ClusterInfo, error) {
	cluster := ClusterInfo{}

	q := r.db.Operator.Core.NewSelect().Model(&cluster).Where(
		"cluster_config = ?", clusterConfig)
	// For backward compatibility: when mode is ConnectModeKubeConfig, also match NULL mode (old data)
	if mode == types.ConnectModeKubeConfig {
		q = q.Where("(mode = ? OR mode IS NULL OR mode = '')", mode)
	} else if mode != "" {
		q = q.Where("mode = ?", mode)
	} else {
		q = q.Where("mode IS NULL OR mode = ''")
	}
	err := q.Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		cluster = ClusterInfo{
			ClusterID:     uuid.New().String(),
			ClusterConfig: clusterConfig,
			Region:        region,
			Enable:        true,
			Mode:          mode,
		}
		_, err = r.db.Operator.Core.NewInsert().Model(&cluster).Exec(ctx)
		if err != nil {
			return nil, err
		}
	}
	return &cluster, err
}

func (r *clusterInfoStoreImpl) AddByClusterID(ctx context.Context, clusterID string, region string) (*ClusterInfo, error) {
	cluster, err := r.ByClusterID(ctx, clusterID)
	if errors.Is(err, sql.ErrNoRows) {
		cluster = ClusterInfo{
			ClusterID:     clusterID,
			ClusterConfig: types.DefaultClusterCongfig,
			Region:        region,
			Enable:        true,
		}
		_, err = r.db.Operator.Core.NewInsert().Model(&cluster).Exec(ctx)
		if err != nil {
			return nil, err
		}
	}
	return &cluster, err
}

func (r *clusterInfoStoreImpl) Update(ctx context.Context, clusterInfo ClusterInfo) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := r.ByClusterConfig(ctx, clusterInfo.ClusterConfig)
		if err == nil {
			return assertAffectedOneRow(tx.NewUpdate().Model(&clusterInfo).WherePK().Exec(ctx))
		}
		return nil
	})
	return err
}

func (r *clusterInfoStoreImpl) UpdateByClusterID(ctx context.Context, event types.ClusterEvent) error {
	event2clusterFunc := func(event types.ClusterEvent, clusterInfo *ClusterInfo) {
		clusterInfo.Region = event.Region
		clusterInfo.ClusterConfig = event.ClusterConfig
		clusterInfo.Zone = event.Zone
		clusterInfo.Provider = event.Provider
		clusterInfo.RunnerEndpoint = event.Endpoint
		clusterInfo.StorageClass = event.StorageClass
		clusterInfo.NetworkInterface = event.NetworkInterface
		clusterInfo.Mode = event.Mode
		clusterInfo.AppEndpoint = event.AppEndpoint
	}

	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		clusterInfo, err := r.ByClusterID(ctx, event.ClusterID)
		if err == nil {
			clusterInfo.Region = event.Region
			clusterInfo.ClusterConfig = event.ClusterConfig
			clusterInfo.Zone = event.Zone
			clusterInfo.Provider = event.Provider
			clusterInfo.RunnerEndpoint = event.Endpoint
			clusterInfo.StorageClass = event.StorageClass
			clusterInfo.NetworkInterface = event.NetworkInterface
			clusterInfo.Mode = event.Mode
			clusterInfo.AppEndpoint = event.AppEndpoint
			return assertAffectedOneRow(r.db.Operator.Core.NewUpdate().Model(&clusterInfo).WherePK().Exec(ctx))
		} else if errors.Is(err, sql.ErrNoRows) {
			clusterInfo = ClusterInfo{}
			clusterInfo.ClusterID = event.ClusterID
			clusterInfo.Enable = true
			event2clusterFunc(event, &clusterInfo)
			_, err = tx.NewInsert().Model(&clusterInfo).Exec(ctx)
			return err
		}
		return err
	})

	return err
}

func (s *clusterInfoStoreImpl) ByClusterID(ctx context.Context, clusterId string) (clusterInfo ClusterInfo, err error) {
	clusterInfo.ClusterID = clusterId
	err = s.db.Operator.Core.NewSelect().Model(&clusterInfo).Where("cluster_id = ?", clusterId).Scan(ctx)
	return
}

func (s *clusterInfoStoreImpl) ByClusterConfig(ctx context.Context, clusterConfig string) (clusterInfo ClusterInfo, err error) {
	clusterInfo.ClusterConfig = clusterConfig
	err = s.db.Operator.Core.NewSelect().Model(&clusterInfo).Where("cluster_config = ?", clusterConfig).Scan(ctx)
	return
}

func (s *clusterInfoStoreImpl) List(ctx context.Context) ([]ClusterInfo, error) {
	var result []ClusterInfo
	_, err := s.db.Operator.Core.NewSelect().Model(&result).Order("region").Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *clusterInfoStoreImpl) BatchUpdateStatus(ctx context.Context, statusEvent []*types.ClusterRes) error {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {

		for _, cluster := range statusEvent {
			_, err := tx.NewUpdate().Model(&ClusterInfo{}).
				Set("Status = ?", types.ClusterStatusRunning).
				Set("resource_status = ?", cluster.ResourceStatus).
				Set("updated_at = now()").
				Where("cluster_id = ?", cluster.ClusterID).
				Exec(ctx)

			if err != nil {
				return errorx.HandleDBError(err, nil)
			}

			for _, nodeRes := range cluster.Resources {
				node := ClusterNode{
					ClusterID:   cluster.ClusterID,
					Name:        nodeRes.NodeName,
					Status:      nodeRes.NodeStatus,
					Labels:      nodeRes.Labels,
					Hardware:    nodeRes.NodeHardware,
					Processes:   nodeRes.Processes,
					ComputeCard: "",
				}

				if nodeRes.NodeHardware.TotalXPU > 0 {
					node.ComputeCard = fmt.Sprintf("%d x %s %s %s",
						nodeRes.NodeHardware.TotalXPU,
						nodeRes.NodeHardware.GPUVendor,
						nodeRes.NodeHardware.XPUModel,
						nodeRes.NodeHardware.XPUMem)
				}

				_, err := tx.NewInsert().Model(&node).
					On("CONFLICT (cluster_id, name) DO UPDATE").
					Set("status = EXCLUDED.status").
					Set("labels = EXCLUDED.labels").
					Set("hardware = EXCLUDED.hardware").
					Set("processes = EXCLUDED.processes").
					Set("compute_card = EXCLUDED.compute_card").
					Set("updated_at = now()").
					Exec(ctx)

				if err != nil {
					return errorx.HandleDBError(err, nil)
				}
			}

			if len(cluster.Resources) > 0 {
				err = s.updateServiceClusterNodes(ctx, tx, cluster)
				if err != nil {
					return errorx.HandleDBError(err, nil)
				}
			}
		}

		return nil
	})

	return err
}

func (s *clusterInfoStoreImpl) updateServiceClusterNodes(ctx context.Context, tx bun.Tx, cluster *types.ClusterRes) error {
	deployNodes := make(map[string]string)
	argoWFNodes := make(map[string]string)

	for _, nodeRes := range cluster.Resources {
		for _, process := range nodeRes.Processes {
			if len(process.DeployID) < 1 || len(process.ClusterNode) < 1 {
				continue
			}
			if len(process.SvcName) > 0 {
				// svcName is unique in a cluster, so we just use svcName as the key.
				nodes := deployNodes[process.SvcName]
				if slices.Contains(strings.Split(nodes, ","), process.ClusterNode) {
					continue
				}
				if len(nodes) > 0 {
					nodes += ","
				}
				nodes += process.ClusterNode
				deployNodes[process.SvcName] = nodes
			}
			if len(process.WorkflowName) > 0 {
				// workflowName is unique in a cluster, so we can use it as the key directly.
				nodes := argoWFNodes[process.WorkflowName]
				if slices.Contains(strings.Split(nodes, ","), process.ClusterNode) {
					continue
				}
				if len(nodes) > 0 {
					nodes += ","
				}
				nodes += process.ClusterNode
				argoWFNodes[process.WorkflowName] = nodes
			}
		}
	}

	for svcName, nodes := range deployNodes {
		_, err := tx.NewUpdate().Model(&Deploy{}).
			Set("cluster_node = ?", nodes).
			Where("svc_name = ?", svcName).Exec(ctx)
		if err != nil {
			return errorx.HandleDBError(err, nil)
		}
	}

	for wfName, nodes := range argoWFNodes {
		_, err := tx.NewUpdate().Model(&ArgoWorkflow{}).
			Set("cluster_node = ?", nodes).
			Where("task_id = ?", wfName).Exec(ctx)
		if err != nil {
			return errorx.HandleDBError(err, nil)
		}
	}

	return nil
}

func (s *clusterInfoStoreImpl) GetClusterResources(ctx context.Context, clusterID string) (*types.ClusterRes, error) {
	clusterInfo, err := s.ByClusterID(ctx, clusterID)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	var clusterNodes []ClusterNode
	err = s.db.Operator.Core.NewSelect().Model(&clusterNodes).Where("cluster_id = ?", clusterID).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	resources := make([]types.NodeResourceInfo, 0, len(clusterNodes))

	for _, node := range clusterNodes {
		resources = append(resources, types.NodeResourceInfo{
			NodeName:     node.Name,
			NodeStatus:   node.Status,
			NodeHardware: node.Hardware,
			Processes:    node.Processes,
			EnableVXPU:   node.EnableVXPU,
			UpdateAt:     node.UpdatedAt.Unix(),
		})
	}

	return &types.ClusterRes{
		ClusterID:      clusterInfo.ClusterID,
		Status:         clusterInfo.Status,
		Region:         clusterInfo.Region,
		Zone:           clusterInfo.Zone,
		Provider:       clusterInfo.Provider,
		StorageClass:   clusterInfo.StorageClass,
		ResourceStatus: types.ResourceStatus(clusterInfo.ResourceStatus),
		Enable:         clusterInfo.Enable,
		NodeNumber:     len(resources),
		Resources:      resources,
		LastUpdateTime: clusterInfo.UpdatedAt.Unix(),
	}, nil
}

func (s *clusterInfoStoreImpl) FindNodeByClusterID(ctx context.Context, clusterID string) ([]ClusterNode, error) {
	var result []ClusterNode
	err := s.db.Operator.Core.NewSelect().Model(&result).Where("cluster_id = ?", clusterID).Scan(ctx, &result)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

func (s *clusterInfoStoreImpl) ListAllNodes(ctx context.Context) ([]ClusterNodeWithRegion, error) {
	var result []ClusterNodeWithRegion
	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("cn.*, ci.region as cluster_region").
		TableExpr("cluster_nodes as cn").
		Join("JOIN cluster_infos ci ON ci.cluster_id = cn.cluster_id").
		Order("cn.cluster_id").
		Order("cn.name").
		Scan(ctx, &result)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

func (s *clusterInfoStoreImpl) GetNodeByID(ctx context.Context, id int64) (*ClusterNodeWithRegion, error) {
	node := &ClusterNodeWithRegion{}
	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("cn.*, ci.region as cluster_region").
		TableExpr("cluster_nodes as cn").
		Join("JOIN cluster_infos ci ON ci.cluster_id = cn.cluster_id").
		Where("cn.id = ?", id).
		Scan(ctx, node)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return node, nil
}

func (s *clusterInfoStoreImpl) UpdateNode(ctx context.Context, id int64, enableVXPU bool) (*ClusterNode, error) {
	node := &ClusterNode{ID: id}
	err := s.db.Operator.Core.NewSelect().Model(node).WherePK().Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}

	node.EnableVXPU = enableVXPU
	_, err = s.db.Operator.Core.NewUpdate().Model(node).WherePK().Exec(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return node, nil
}

func (s *clusterInfoStoreImpl) GetClusterNodeByID(ctx context.Context, id int64) (*ClusterNode, error) {
	var node ClusterNode
	err := s.db.Operator.Core.NewSelect().Model(&node).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &node, nil
}

func (s *clusterInfoStoreImpl) UpdateClusterNodeByNode(ctx context.Context, node ClusterNode) error {
	_, err := s.db.Operator.Core.NewUpdate().Model(&node).WherePK().Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *clusterInfoStoreImpl) AddNodeOwnership(ctx context.Context, ownership ClusterNodeOwnership) error {
	_, err := s.db.Operator.Core.NewInsert().Model(&ownership).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *clusterInfoStoreImpl) DeleteNodeOwnership(ctx context.Context, clusterNodeID int64) error {
	_, err := s.db.Operator.Core.NewDelete().Model(&ClusterNodeOwnership{}).Where("cluster_node_id = ?", clusterNodeID).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *clusterInfoStoreImpl) GetNodeOwnership(ctx context.Context, clusterNodeID int64) (*ClusterNodeOwnership, error) {
	var ownership ClusterNodeOwnership
	err := s.db.Operator.Core.NewSelect().Model(&ownership).Where("cluster_node_id = ?", clusterNodeID).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &ownership, nil
}

func (s *clusterInfoStoreImpl) UpdateNodeOwnership(ctx context.Context, ownership ClusterNodeOwnership) error {
	_, err := s.db.Operator.Core.NewUpdate().Model(&ownership).WherePK().Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *clusterInfoStoreImpl) ExecuteInTx(ctx context.Context, fn func(ctx context.Context, store ClusterInfoStore) error) error {
	return s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		txStore := &clusterInfoStoreImpl{
			db: &DB{
				Operator: tx,
				BunDB:    s.db.BunDB,
			},
		}
		return fn(ctx, txStore)
	})
}
