package database

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

type SpaceStore struct {
	db *DB
}

func NewSpaceStore() *SpaceStore {
	return &SpaceStore{
		db: defaultDB,
	}
}

func (s *SpaceStore) BeginTx(ctx context.Context) (bun.Tx, error) {
	return s.db.Core.BeginTx(ctx, nil)
}

func (s *SpaceStore) CreateTx(ctx context.Context, tx bun.Tx, input Space) (*Space, error) {
	res, err := tx.NewInsert().Model(&input).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create space in tx failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create space in tx failed,error:%w", err)
	}

	input.ID, _ = res.LastInsertId()
	return &input, nil
}

func (s *SpaceStore) Create(ctx context.Context, input Space) (*Space, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create space in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create space in db failed,error:%w", err)
	}

	input.ID, _ = res.LastInsertId()
	return &input, nil
}

func (s *SpaceStore) Update(ctx context.Context, input Space) (err error) {
	input.UpdatedAt = time.Now()
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *SpaceStore) PublicToUser(ctx context.Context, userID int64, search, sort string, per, page int) ([]Space, int, error) {
	var (
		spaces []Space
		count  int
		err    error
	)
	query := s.db.Operator.Core.
		NewSelect().
		Model(&spaces).
		Relation("Repository").
		Relation("Resource").
		Relation("Sdk")
		// Relation("User")

	if userID > 0 {
		query = query.Where("repository.private = ? or repository.user_id = ?", false, userID)
	} else {
		query = query.Where("repository.private = ?", false)
	}

	if search != "" {
		search = strings.ToLower(search)
		query = query.Where(
			"LOWER(repository.path) like ? or LOWER(repository.name) like ?",
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
		)
	}

	count, err = query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	query = query.Order(sortBy[sort])
	query = query.Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return nil, 0, err
	}
	return spaces, count, nil
}

func (s *SpaceStore) FindByPath(ctx context.Context, namespace, name string) (*Space, error) {
	resSpace := new(Space)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resSpace).
		Relation("Repository.User").
		Where("repository.path =?", fmt.Sprintf("%s/%s", namespace, name)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find space: %w", err)
	}

	return resSpace, err
}

func (s *SpaceStore) Delete(ctx context.Context, input Space) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete space in tx failed,error:%w", err)
	}
	return nil
}
