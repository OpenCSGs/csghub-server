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

type Space struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	Name          string      `bun:",notnull" json:"name"`
	UrlSlug       string      `bun:",notnull" json:"nickname"`
	Likes         int64       `bun:",notnull" json:"likes"`
	Downloads     int64       `bun:",notnull" json:"downloads"`
	Path          string      `bun:",notnull" json:"path"`
	GitPath       string      `bun:",notnull" json:"git_path"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	Private       bool        `bun:",notnull" json:"private"`
	UserID        int64       `bun:",notnull" json:"user_id"`
	User          *User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	// gradio, streamlit, docker etc
	Sdk string `bun:",notnull" json:"sdk"`
	times
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

func (s *SpaceStore) PublicToUser(ctx context.Context, userID int64, search, sort string, per, page int) ([]Space, int, error) {
	var (
		spaces []Space
		count  int
		err    error
	)
	query := s.db.Operator.Core.
		NewSelect().
		Model(&spaces).
		Relation("User")

	if userID > 0 {
		query = query.Where("space.private = ? or space.user_id = ?", false, userID)
	} else {
		query = query.Where("space.private = ?", false)
	}

	if search != "" {
		search = strings.ToLower(search)
		query = query.Where(
			"LOWER(space.path) like ? or LOWER(space.name) like ?",
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
