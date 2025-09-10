package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"

	corev1 "k8s.io/api/core/v1"
)

type knativeServiceImpl struct {
	db *DB
}

type KnativeServiceStore interface {
	Get(ctx context.Context, svcName, clusterID string) (*KnativeService, error)
	GetByCluster(ctx context.Context, clusterID string) ([]KnativeService, error)
	Add(ctx context.Context, service *KnativeService) error
	Update(ctx context.Context, service *KnativeService) error
	Delete(ctx context.Context, clusterID, svcName string) error
}

func NewKnativeServiceStore() KnativeServiceStore {
	return &knativeServiceImpl{
		db: defaultDB,
	}
}

func NewKnativeServiceWithDB(db *DB) KnativeServiceStore {
	return &knativeServiceImpl{
		db: db,
	}
}

type KnativeService struct {
	ID             int64                  `bun:",pk,autoincrement" json:"id"`
	Name           string                 `bun:",notnull" json:"name"`
	Status         corev1.ConditionStatus `bun:",notnull" json:"status"`
	Code           int                    `bun:",notnull" json:"code"`
	ClusterID      string                 `bun:",notnull" json:"cluster_id"`
	Endpoint       string                 `bun:"," json:"endpoint"`
	ActualReplica  int                    `bun:"," json:"actual_replica"`
	DesiredReplica int                    `bun:"," json:"desired_replica"`
	Instances      []types.Instance       `bun:"type:jsonb" json:"instances,omitempty"`
	UserUUID       string                 `bun:"," json:"user_uuid"`
	DeployID       int64                  `bun:"," json:"deploy_id"`
	DeployType     int                    `bun:"," json:"deploy_type"`
	DeploySKU      string                 `bun:"," json:"deploy_sku"`
	OrderDetailID  int64                  `bun:"," json:"order_detail_id"`
	TaskID         int64                  `bun:"," json:"task_id"`
	times
}

// get
func (s *knativeServiceImpl) Get(ctx context.Context, svcName, clusterID string) (*KnativeService, error) {
	var service KnativeService
	var err error
	if clusterID == "" {
		// backward compatibility, some space has no cluster id
		err = s.db.Operator.Core.NewSelect().Model(&service).Where("name = ?", svcName).Scan(ctx)
	} else {
		err = s.db.Operator.Core.NewSelect().Model(&service).Where("name = ? and cluster_id = ?", svcName, clusterID).Scan(ctx)
	}
	return &service, err
}

// add
func (s *knativeServiceImpl) Add(ctx context.Context, service *KnativeService) error {
	_, err := s.db.Operator.Core.NewInsert().Model(service).On("CONFLICT(name, cluster_id) DO UPDATE").Exec(ctx)
	return err
}

// update
func (s *knativeServiceImpl) Update(ctx context.Context, service *KnativeService) error {
	_, err := s.db.Operator.Core.NewUpdate().Model(service).WherePK().Exec(ctx)
	return err
}

// delete
func (s *knativeServiceImpl) Delete(ctx context.Context, clusterID, svcName string) error {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewDelete().Model(&KnativeService{}).
			Where("name = ? and cluster_id = ?", svcName, clusterID).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("delete cluster service %s error: %w", svcName, err)
		}

		_, err = tx.NewDelete().Model(&DeployLog{}).
			Where("cluster_id = ? and svc_name = ?", clusterID, svcName).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("delete deploy service %s log error: %w", svcName, err)
		}

		return nil
	})

	return err
}

// GetByCluster
func (s *knativeServiceImpl) GetByCluster(ctx context.Context, clusterID string) ([]KnativeService, error) {
	var services []KnativeService
	err := s.db.Operator.Core.NewSelect().Model(&services).Where("cluster_id = ?", clusterID).Scan(ctx)
	return services, err
}
