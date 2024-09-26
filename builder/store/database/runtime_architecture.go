package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type RuntimeArchitecturesStore struct {
	db *DB
}

func NewRuntimeArchitecturesStore() *RuntimeArchitecturesStore {
	return &RuntimeArchitecturesStore{
		db: defaultDB,
	}
}

type RuntimeArchitecture struct {
	ID                 int64  `bun:",pk,autoincrement" json:"id"`
	RuntimeFrameworkID int64  `bun:",notnull" json:"runtime_framework_id"`
	ArchitectureName   string `bun:",notnull" json:"architecture_name"`
}

func (ra *RuntimeArchitecturesStore) ListByRuntimeFrameworkID(ctx context.Context, id int64) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.NewSelect().Model(&result).Where("runtime_framework_id = ?", id).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	return result, nil
}

func (ra *RuntimeArchitecturesStore) Add(ctx context.Context, arch RuntimeArchitecture) error {
	res, err := ra.db.Core.NewInsert().Model(&arch).Exec(ctx, &arch)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("creating runtime architecture in the db failed,error:%w", err)
	}
	return nil
}

func (ra *RuntimeArchitecturesStore) DeleteByRuntimeIDAndArchName(ctx context.Context, id int64, archName string) error {
	var arch RuntimeArchitecture
	_, err := ra.db.Core.NewDelete().Model(&arch).Where("runtime_framework_id = ? and architecture_name = ?", id, archName).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleteing runtime architecture in the db failed, error:%w", err)
	}
	return nil
}

func (ra *RuntimeArchitecturesStore) FindByRuntimeIDAndArchName(ctx context.Context, id int64, archName string) (*RuntimeArchitecture, error) {
	var arch RuntimeArchitecture
	_, err := ra.db.Core.NewSelect().Model(&arch).Where("runtime_framework_id = ? and architecture_name = ?", id, archName).Exec(ctx, &arch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting runtime architecture in the db failed, error:%w", err)
	}
	return &arch, nil
}

func (ra *RuntimeArchitecturesStore) ListByRArchName(ctx context.Context, archName string) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.NewSelect().Model(&result).Where("architecture_name = ?", archName).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	return result, nil
}
