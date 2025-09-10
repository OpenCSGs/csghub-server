package database

import (
	"context"
	"fmt"
	"time"

	"opencsg.com/csghub-server/common/types"
)

type LicenseStore interface {
	List(ctx context.Context, req types.QueryLicenseReq) ([]License, int, error)
	Create(ctx context.Context, input License) error
	GetByID(ctx context.Context, id int64) (*License, error)
	Update(ctx context.Context, input License) error
	GetLatestActive(ctx context.Context) (*License, error)
	Delete(ctx context.Context, input License) error
}

type licenseStoreImpl struct {
	db *DB
}

type License struct {
	ID         int64     `bun:",pk,autoincrement" json:"id"`
	Key        string    `bun:",notnull" json:"key"`
	Company    string    `bun:",notnull" json:"company"`
	Email      string    `bun:",notnull" json:"email"`
	Product    string    `bun:",notnull" json:"product"`
	Edition    string    `bun:",notnull" json:"edition"`
	Version    string    `bun:",nullzero" json:"version"`
	Status     string    `bun:",nullzero" json:"status"`
	MaxUser    int       `bun:",notnull" json:"max_user"`
	StartTime  time.Time `bun:",notnull" json:"start_time"`
	ExpireTime time.Time `bun:",notnull" json:"expire_time"`
	Extra      string    `bun:",nullzero" json:"extra"`
	Remark     string    `bun:",nullzero" json:"remark"`
	UserUUID   string    `bun:",notnull" json:"user_uuid"`
	times
}

func NewLicenseStore() LicenseStore {
	return &licenseStoreImpl{db: defaultDB}
}

func NewLicenseStoreWithDB(db *DB) LicenseStore {
	return &licenseStoreImpl{db: db}
}

func (s *licenseStoreImpl) List(ctx context.Context, req types.QueryLicenseReq) ([]License, int, error) {
	var licenses []License
	query := s.db.Operator.Core.NewSelect().Model(&licenses)

	if req.Product != "" {
		query = query.Where("product = ?", req.Product)
	}

	if req.Edition != "" {
		query = query.Where("edition  = ?", req.Edition)
	}

	if req.Search != "" {
		query = query.Where("company LIKE ? OR email LIKE ? OR remark LIKE ?",
			fmt.Sprintf("%%%s%%", req.Search),
			fmt.Sprintf("%%%s%%", req.Search),
			fmt.Sprintf("%%%s%%", req.Search))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count license with error: %w", err)
	}

	query = query.Order("id DESC").Limit(req.Per).Offset((req.Page - 1) * req.Per)

	err = query.Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("select license with error: %w", err)
	}

	return licenses, total, nil
}

// Create implements LicenseStoreInterface.
func (s *licenseStoreImpl) Create(ctx context.Context, input License) error {
	_, err := s.db.Operator.Core.NewInsert().Model(&input).Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert license with error: %w", err)
	}
	return nil
}

// GetByID implements LicenseStoreInterface.
func (s *licenseStoreImpl) GetByID(ctx context.Context, id int64) (*License, error) {
	var license License
	err := s.db.Operator.Core.NewSelect().Model(&license).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select license by id %d with error: %w", id, err)
	}
	return &license, nil
}

func (s *licenseStoreImpl) Update(ctx context.Context, input License) error {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("update license with error: %w", err)
	}
	return nil
}

func (s *licenseStoreImpl) Delete(ctx context.Context, input License) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete license failed,error:%w", err)
	}
	return nil
}

func (s *licenseStoreImpl) GetLatestActive(ctx context.Context) (*License, error) {
	var license License
	err := s.db.Operator.Core.NewSelect().Model(&license).
		Where("start_time <= NOW() AND expire_time >= NOW()").
		Order("id DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select latest active license with error: %w", err)
	}
	return &license, nil
}
