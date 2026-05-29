package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/looplab/fsm"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type DatasetApplication struct {
	ID               int64                          `bun:"id,pk,autoincrement" json:"id"`
	DatasetID        int64                          `bun:"dataset_id,notnull" json:"dataset_id"`
	Dataset          *Dataset                       `bun:"rel:belongs-to,join:dataset_id=id" json:"dataset"`
	ApplicantID      int64                          `bun:"applicant_id,notnull" json:"applicant_id"`
	Applicant        *User                          `bun:"rel:belongs-to,join:applicant_id=id" json:"applicant"`
	Action           types.DatasetApplicationAction `bun:"action,notnull" json:"action"`
	Price            float64                        `bun:"price" json:"price"`
	RelatedDatasetID int64                          `bun:"related_dataset_id" json:"related_dataset_id"`
	RelatedDataset   *Dataset                       `bun:"rel:belongs-to,join:related_dataset_id=id" json:"related_dataset"`
	Status           types.DatasetApplicationStatus `bun:"status,notnull,default:'pending'" json:"status"`
	ReviewerID       int64                          `bun:"reviewer_id" json:"reviewer_id"`
	Reviewer         *User                          `bun:"rel:belongs-to,join:reviewer_id=id" json:"reviewer"`
	ReviewMsg        string                         `bun:"review_msg" json:"review_msg"`
	times
}

// DatasetApplication FSM events
const (
	AppEventApprove = "approve"
	AppEventReject  = "reject"
)

type DatasetApplicationWithFSM struct {
	application *DatasetApplication
	from        types.DatasetApplicationStatus
	fsm         *fsm.FSM
}

func NewDatasetApplicationWithFSM(app *DatasetApplication) DatasetApplicationWithFSM {
	return DatasetApplicationWithFSM{
		application: app,
		from:        app.Status,
		fsm: fsm.NewFSM(
			string(app.Status),
			fsm.Events{
				{
					Name: AppEventApprove,
					Src:  []string{string(types.DatasetApplicationStatusPending)},
					Dst:  string(types.DatasetApplicationStatusApproved),
				},
				{
					Name: AppEventReject,
					Src:  []string{string(types.DatasetApplicationStatusPending)},
					Dst:  string(types.DatasetApplicationStatusRejected),
				},
			},
			fsm.Callbacks{
				"entry_state": func(ctx context.Context, event *fsm.Event) {
					app.Status = types.DatasetApplicationStatus(event.Dst)
				},
			},
		),
	}
}

func (a *DatasetApplicationWithFSM) SubmitEvent(ctx context.Context, event string) bool {
	res := a.fsm.Event(ctx, event)
	if res == nil {
		return true
	}
	var noTrans fsm.NoTransitionError
	return errors.As(res, &noTrans) && noTrans.Err == nil
}

func (a *DatasetApplicationWithFSM) Current() string {
	return a.fsm.Current()
}

type DatasetApplicationStore interface {
	Create(ctx context.Context, input DatasetApplication) (*DatasetApplication, error)
	Update(ctx context.Context, input DatasetApplication) error
	FindByID(ctx context.Context, id int64) (*DatasetApplication, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*DatasetApplication, error)
	FindByDatasetID(ctx context.Context, datasetID int64) ([]*DatasetApplication, error)
	FindPendingByDatasetID(ctx context.Context, datasetID int64) (*DatasetApplication, error)
	List(ctx context.Context, status, search string, per, page int) ([]*DatasetApplication, int, error)
	// CreateApplicationAndLinkDataset creates an application and updates dataset's CurrentApplicationID in a transaction with row lock
	CreateApplicationAndLinkDataset(ctx context.Context, app DatasetApplication) (*DatasetApplication, error)
	// ReviewApplication executes the full review in a transaction with row locks.
	// onApprove is called when status=approved to let the caller inspect the result.
	ReviewApplication(ctx context.Context, appID int64, reviewerID int64, reviewMsg string, action string, onApprove func(app *DatasetApplication) error) (*DatasetApplication, error)
}

type datasetApplicationStoreImpl struct {
	db *DB
}

func NewDatasetApplicationStore() DatasetApplicationStore {
	return &datasetApplicationStoreImpl{db: defaultDB}
}

func NewDatasetApplicationStoreWithDB(db *DB) DatasetApplicationStore {
	return &datasetApplicationStoreImpl{db: db}
}

func (s *datasetApplicationStoreImpl) Create(ctx context.Context, input DatasetApplication) (*DatasetApplication, error) {
	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create dataset application in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create dataset application in db failed, error: %w", err)
	}

	return &input, nil
}

func (s *datasetApplicationStoreImpl) Update(ctx context.Context, input DatasetApplication) error {
	input.UpdatedAt = time.Now()
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return err
}

func (s *datasetApplicationStoreImpl) FindByID(ctx context.Context, id int64) (*DatasetApplication, error) {
	var app DatasetApplication
	err := s.db.Operator.Core.NewSelect().
		Model(&app).
		Where("dataset_application.id = ?", id).
		Relation("Dataset.Repository").
		Relation("Dataset.RelatedDataset").
		Relation("Dataset.RelatedDataset.Repository").
		Relation("RelatedDataset").
		Relation("RelatedDataset.Repository").
		Relation("Applicant").
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset application by id: %d, error: %w", id, err)
	}

	return &app, nil
}

func (s *datasetApplicationStoreImpl) FindByDatasetID(ctx context.Context, datasetID int64) ([]*DatasetApplication, error) {
	var apps []*DatasetApplication
	err := s.db.Operator.Core.NewSelect().
		Model(&apps).
		Where("dataset_id = ?", datasetID).
		Relation("Applicant").
		Relation("Reviewer").
		Relation("Dataset").
		Relation("Dataset.Repository").
		Relation("RelatedDataset").
		Relation("RelatedDataset.Repository").
		Order("created_at DESC").
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("dataset_id", datasetID))
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset applications by dataset id: %d, error: %w", datasetID, err)
	}

	return apps, nil
}

func (s *datasetApplicationStoreImpl) FindPendingByDatasetID(ctx context.Context, datasetID int64) (*DatasetApplication, error) {
	var app DatasetApplication
	err := s.db.Operator.Core.NewSelect().
		Model(&app).
		Where("dataset_id = ? AND status = ?", datasetID, types.DatasetApplicationStatusPending).
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("dataset_id", datasetID))
	if err != nil {
		return nil, fmt.Errorf("failed to find pending dataset application by dataset id: %d, error: %w", datasetID, err)
	}

	return &app, nil
}

func (s *datasetApplicationStoreImpl) List(ctx context.Context, status, search string, per, page int) ([]*DatasetApplication, int, error) {
	var apps []*DatasetApplication
	query := s.db.Operator.Core.NewSelect().
		Model(&apps).
		Relation("Applicant").
		Relation("Reviewer").
		Relation("Dataset.Repository")
	if status != "" {
		query = query.Where("dataset_application.status = ?", status)
	}
	if search != "" {
		query = query.
			Join("JOIN datasets ON datasets.id = dataset_application.dataset_id").
			Join("JOIN repositories ON repositories.id = datasets.repository_id").
			Where("LOWER(repositories.name) LIKE ? OR LOWER(repositories.path) LIKE ?",
				"%"+strings.ToLower(search)+"%", "%"+strings.ToLower(search)+"%")
	}
	query = query.Order("dataset_application.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err := query.Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list dataset applications, error: %w", err)
	}
	total, err := query.Count(ctx)
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count dataset applications, error: %w", err)
	}

	return apps, total, nil
}

func (s *datasetApplicationStoreImpl) FindByIDForUpdate(ctx context.Context, id int64) (*DatasetApplication, error) {
	var app DatasetApplication
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Lock the application row first (no joins, FOR UPDATE works)
		if err := tx.NewSelect().
			Model(&app).
			Where("dataset_application.id = ?", id).
			For("UPDATE").
			Scan(ctx); err != nil {
			return err
		}
		// Load relations separately after locking
		if err := tx.NewSelect().
			Model(&app).
			WherePK().
			Relation("Dataset.Repository").
			Relation("Applicant").
			Scan(ctx); err != nil {
			return err
		}
		return nil
	})
	err = errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset application for update by id: %d, error: %w", id, err)
	}
	return &app, nil
}

func (s *datasetApplicationStoreImpl) CreateApplicationAndLinkDataset(ctx context.Context, app DatasetApplication) (*DatasetApplication, error) {
	var result DatasetApplication
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Lock the dataset row to serialize CurrentApplicationID updates
		var ds Dataset
		if err := tx.NewSelect().Model(&ds).Where("id = ?", app.DatasetID).For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("failed to lock dataset row: %w", err)
		}

		app.CreatedAt = time.Now()
		app.UpdatedAt = time.Now()
		if _, err := tx.NewInsert().Model(&app).Exec(ctx, &app); err != nil {
			return fmt.Errorf("failed to create dataset application: %w", err)
		}

		ds.CurrentApplicationID = app.ID
		ds.LastUpdatedAt = time.Now()
		if _, err := tx.NewUpdate().Model(&ds).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("failed to update dataset current application: %w", err)
		}

		result = app
		return nil
	})
	if err != nil {
		slog.Error("create dataset application with link in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create dataset application with link in db failed, error: %w", err)
	}
	return &result, nil
}

func (s *datasetApplicationStoreImpl) ReviewApplication(ctx context.Context, appID int64, reviewerID int64, reviewMsg string, action string, onApprove func(app *DatasetApplication) error) (*DatasetApplication, error) {
	var result DatasetApplication
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Lock the application row
		var app DatasetApplication
		if err := tx.NewSelect().
			Model(&app).
			Where("dataset_application.id = ?", appID).
			For("UPDATE").
			Scan(ctx); err != nil {
			return fmt.Errorf("failed to lock application row: %w", err)
		}

		// Load relations after locking
		if err := tx.NewSelect().
			Model(&app).
			WherePK().
			Relation("Dataset.Repository").
			Relation("Applicant").
			Relation("Reviewer").
			Scan(ctx); err != nil {
			return fmt.Errorf("failed to load application relations: %w", err)
		}

		// Apply application FSM
		var event string
		switch action {
		case "approve":
			event = AppEventApprove
		case "reject":
			event = AppEventReject
		default:
			return fmt.Errorf("invalid review action: %s", action)
		}

		appFSM := NewDatasetApplicationWithFSM(&app)
		if !appFSM.SubmitEvent(ctx, event) {
			return fmt.Errorf("application status %s does not allow event %s", app.Status, event)
		}
		app.Status = types.DatasetApplicationStatus(appFSM.Current())
		app.ReviewerID = reviewerID
		app.ReviewMsg = reviewMsg

		// On approve, update dataset and repository
		if action == "approve" {
			var ds Dataset
			if err := tx.NewSelect().Model(&ds).Where("dataset.id = ?", app.DatasetID).For("UPDATE").Scan(ctx); err != nil {
				return fmt.Errorf("failed to lock dataset: %w", err)
			}
			// Load repository relation separately after locking
			if err := tx.NewSelect().Model(&ds).WherePK().Relation("Repository").Scan(ctx); err != nil {
				return fmt.Errorf("failed to load dataset repository: %w", err)
			}

			var dsEvent string
			switch app.Action {
			case types.DatasetApplicationActionInitial, types.DatasetApplicationActionEdit, types.DatasetApplicationActionRelist:
				dsEvent = DatasetEventList
				if app.Price > 0 {
					ds.Price = app.Price
				}
				if app.RelatedDatasetID > 0 {
					ds.RelatedDatasetID = app.RelatedDatasetID
				}
				ds.DatasetType = types.DatasetTypeCommercial
				ds.Repository.Private = false
			case types.DatasetApplicationActionDelist:
				dsEvent = DatasetEventDelist
				ds.DatasetType = types.DatasetTypeNormal
				ds.Price = 0
				ds.RelatedDatasetID = 0
			}

			dsFSM := NewDatasetWithFSM(&ds)
			if !dsFSM.SubmitEvent(ctx, dsEvent) {
				return fmt.Errorf("dataset status %s does not allow event %s", ds.Status, dsEvent)
			}
			ds.Status = types.DatasetStatus(dsFSM.Current())

			ds.LastUpdatedAt = time.Now()
			if _, err := tx.NewUpdate().Model(&ds).WherePK().Exec(ctx); err != nil {
				return fmt.Errorf("failed to update dataset: %w", err)
			}
			if _, err := tx.NewUpdate().Model(ds.Repository).WherePK().Exec(ctx); err != nil {
				return fmt.Errorf("failed to update repository: %w", err)
			}
		}

		app.UpdatedAt = time.Now()
		if _, err := tx.NewUpdate().Model(&app).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("failed to update application: %w", err)
		}

		result = app
		return nil
	})
	if err != nil {
		return nil, err
	}

	if onApprove != nil {
		if err := onApprove(&result); err != nil {
			return nil, err
		}
	}
	return &result, nil
}
