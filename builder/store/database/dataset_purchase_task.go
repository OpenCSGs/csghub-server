package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/common/types"
)

// DatasetPurchaseTask represents a dataset purchase task
type DatasetPurchaseTask struct {
	ID               int64                           `bun:"id,pk,autoincrement" json:"id"`
	DatasetPath      string                          `bun:"dataset_path,notnull" json:"dataset_path"`
	DatasetID        int64                           `bun:"dataset_id,notnull" json:"dataset_id"`
	RelatedDatasetID int64                           `bun:"related_dataset_id,notnull" json:"related_dataset_id"`
	PurchaserID      int64                           `bun:"purchaser_id,notnull" json:"purchaser_id"`
	Purchaser        *User                           `bun:"rel:belongs-to,join:purchaser_id=id" json:"purchaser"`
	TaskStatus       types.DatasetPurchaseTaskStatus `bun:"task_status,notnull" json:"task_status"`
	times
}

type DatasetPurchaseTaskStore interface {
	Create(ctx context.Context, input DatasetPurchaseTask) (*DatasetPurchaseTask, error)
	Update(ctx context.Context, input DatasetPurchaseTask) (err error)
	FindByID(ctx context.Context, taskID int64) (*DatasetPurchaseTask, error)
	FindByDatasetID(ctx context.Context, datasetID int64) (*DatasetPurchaseTask, error)
	FindByRelatedDatasetID(ctx context.Context, relatedDatasetID int64) (*DatasetPurchaseTask, error)
	FindByPurchaserID(ctx context.Context, purchaserID int64) ([]*DatasetPurchaseTask, error)
}

type datasetPurchaseTaskStoreImpl struct {
	db *DB
}

func NewDatasetPurchaseTaskStore() DatasetPurchaseTaskStore {
	return &datasetPurchaseTaskStoreImpl{db: defaultDB}
}

func NewDatasetPurchaseTaskStoreWithDB(db *DB) DatasetPurchaseTaskStore {
	return &datasetPurchaseTaskStoreImpl{db: db}
}

func (s *datasetPurchaseTaskStoreImpl) Create(ctx context.Context, input DatasetPurchaseTask) (*DatasetPurchaseTask, error) {
	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create dataset purchase task in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create dataset purchase task in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *datasetPurchaseTaskStoreImpl) Update(ctx context.Context, input DatasetPurchaseTask) (err error) {
	input.UpdatedAt = time.Now()
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *datasetPurchaseTaskStoreImpl) FindByDatasetID(ctx context.Context, datasetID int64) (*DatasetPurchaseTask, error) {
	var task DatasetPurchaseTask
	err := s.db.Operator.Core.NewSelect().
		Model(&task).
		Where("dataset_id = ?", datasetID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset purchase task by dataset id: %d, error: %w", datasetID, err)
	}

	return &task, nil
}

func (s *datasetPurchaseTaskStoreImpl) FindByRelatedDatasetID(ctx context.Context, relatedDatasetID int64) (*DatasetPurchaseTask, error) {
	var task DatasetPurchaseTask
	err := s.db.Operator.Core.NewSelect().
		Model(&task).
		Where("related_dataset_id = ?", relatedDatasetID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset purchase task by related dataset id: %d, error: %w", relatedDatasetID, err)
	}

	return &task, nil
}

func (s *datasetPurchaseTaskStoreImpl) FindByID(ctx context.Context, taskID int64) (*DatasetPurchaseTask, error) {
	var task DatasetPurchaseTask
	err := s.db.Operator.Core.NewSelect().
		Model(&task).
		Where("id = ?", taskID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset purchase task by id: %d, error: %w", taskID, err)
	}

	return &task, nil
}

func (s *datasetPurchaseTaskStoreImpl) FindByPurchaserID(ctx context.Context, purchaserID int64) ([]*DatasetPurchaseTask, error) {
	var tasks []*DatasetPurchaseTask
	err := s.db.Operator.Core.NewSelect().
		Model(&tasks).
		Where("purchaser_id = ?", purchaserID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset purchase tasks by purchaser id: %d, error: %w", purchaserID, err)
	}

	return tasks, nil
}
