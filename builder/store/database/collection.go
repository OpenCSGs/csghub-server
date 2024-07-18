package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type CollectionStore struct {
	db *DB
}

func NewCollectionStore() *CollectionStore {
	return &CollectionStore{
		db: defaultDB,
	}
}

type Collection struct {
	ID           int64        `bun:",pk,autoincrement" json:"id"`
	Username     string       `bun:",notnull" json:"username"`
	UserID       int64        `bun:",notnull" json:"user_id"`
	Name         string       `bun:",notnull" json:"name"`
	Theme        string       `bun:",notnull" json:"theme"`
	Nickname     string       `bun:",notnull" json:"nickname"`
	Description  string       `bun:",nullzero" json:"description"`
	Private      bool         `bun:",notnull" json:"private"`
	Repositories []Repository `bun:"m2m:collection_repositories,join:Collection=Repository" json:"repositories"`
	Likes        int64        `bun:",nullzero" json:"likes"`
	times
}

type CollectionRepository struct {
	ID           int64       `bun:",autoincrement" json:"id"`
	CollectionID int64       `bun:",pk" json:"collection_id"`
	RepositoryID int64       `bun:",pk" json:"repository_id"`
	Collection   *Collection `bun:"rel:belongs-to,join:collection_id=id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id"`
}

var Fields = []string{"id", "download_count", "likes", "path", "private", "repository_type", "updated_at", "created_at", "user_id", "name", "nickname", "description"}

// query collections in the database
func (cs *CollectionStore) GetCollections(ctx context.Context, filter *types.CollectionFilter, per, page int) (collections []Collection, total int, err error) {
	if filter.Sort == "trending" {
		return cs.QueryByTrending(ctx, filter, per, page)
	}
	query := cs.db.Operator.Core.
		NewSelect().
		Model(&collections).
		Relation("Repositories.Tags", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("category = ?", "task")
		}).
		Relation("Repositories", func(q *bun.SelectQuery) *bun.SelectQuery {
			q.Column(Fields...).OrderExpr("updated_at DESC").Limit(3)
			return q
		}).
		Where("private =  ?", false)
	if filter.Search != "" {
		filter.Search = strings.ToLower(filter.Search)
		query.Where(
			"LOWER(name) like ?", fmt.Sprintf("%%%s%%", filter.Search),
		)
	}
	err = query.Order(sortBy[filter.Sort]).
		Limit(per).Offset((page - 1) * per).
		Scan(ctx)
	if err != nil {
		return nil, 0, err
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

// query collections in the database
func (cs *CollectionStore) QueryByTrending(ctx context.Context, filter *types.CollectionFilter, per, page int) (collections []Collection, total int, err error) {
	query := cs.db.Operator.Core.NewSelect().
		Model(&collections).
		Column("collection.*").
		ColumnExpr("SUM(COALESCE(rors.score, 0)+COALESCE(ropw.weight, 0)) AS popularity").
		Join("LEFT JOIN collection_repositories cr ON collection.id = cr.collection_id ").
		Join("LEFT JOIN repositories r ON cr.repository_id = r.id").
		Join("LEFT JOIN recom_op_weights ropw ON r.id = ropw.repository_id").
		Join("LEFT JOIN recom_repo_scores rors ON r.id = rors.repository_id")
	query.Where("collection.private = ?", false)
	if filter.Search != "" {
		filter.Search = strings.ToLower(filter.Search)
		query.Where(
			"LOWER(collection.name) like ?", fmt.Sprintf("%%%s%%", filter.Search),
		)
	}
	query.Group("collection.id")
	err = query.Order(sortBy[filter.Sort]).
		Limit(per).Offset((page - 1) * per).
		Scan(ctx)
	fmt.Println(query.String())
	if err != nil {
		return nil, 0, err
	}

	total, err = query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]interface{}, 0)
	for _, collection := range collections {
		ids = append(ids, collection.ID)
	}
	err = cs.db.Operator.Core.
		NewSelect().
		Model(&collections).
		Relation("Repositories.Tags", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("category = ?", "task")
		}).
		Relation("Repositories", func(q *bun.SelectQuery) *bun.SelectQuery {
			q.Column(Fields...).OrderExpr("updated_at DESC").Limit(3)
			return q
		}).
		OrderExpr("array_position(ARRAY[?]::int[], id::int)", ids...).
		Where("id IN (?)", bun.In(ids)).Scan(ctx)

	if err != nil {
		return nil, 0, err
	}

	return collections, total, nil
}

func (cs *CollectionStore) CreateCollection(ctx context.Context, collection Collection) (*Collection, error) {
	res, err := cs.db.Core.NewInsert().Model(&collection).Exec(ctx, &collection)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("failed to create collection in db, error:%w", err)
	}

	return &collection, nil
}

func (cs *CollectionStore) DeleteCollection(ctx context.Context, id int64, uid int64) error {
	var collection Collection
	res, err := cs.db.Operator.Core.NewDelete().Model(&collection).Where("id =?", id).Where("user_id =?", uid).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to delete collection in db, error:%w", err)
	}
	return nil
}

func (cs *CollectionStore) UpdateCollection(ctx context.Context, collection Collection) (*Collection, error) {

	_, err := cs.db.Core.NewUpdate().Model(&collection).WherePK().Exec(ctx)
	return &collection, err
}

func (cs *CollectionStore) GetCollection(ctx context.Context, id int64) (*Collection, error) {
	collection := new(Collection)
	err := cs.db.Operator.Core.
		NewSelect().
		Model(collection).
		Relation("Repositories.Tags", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("category = ?", "task")
		}).
		Relation("Repositories", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Column(Fields...).OrderExpr("updated_at DESC")
		}).
		Where("id =?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("can not find collection: %w", err)
	}

	return collection, err
}

func (cs *CollectionStore) ByUserLikes(ctx context.Context, userID int64, per, page int) (collections []Collection, total int, err error) {
	query := cs.db.Operator.Core.
		NewSelect().
		Model(&collections).
		Relation("Repositories.Tags", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("category = ?", "task")
		}).
		Relation("Repositories", func(q *bun.SelectQuery) *bun.SelectQuery {
			q.Column(Fields...).OrderExpr("updated_at DESC").Limit(3)
			return q
		}).
		Where("collection.id in (select collection_id from user_likes where user_id=?)", userID)

	query = query.Order("collection.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (cs *CollectionStore) FindById(ctx context.Context, id int64) (collection Collection, err error) {
	q := cs.db.Operator.Core.
		NewSelect()
	err = q.
		Model(&collection).
		Where("id =?", id).
		Scan(ctx)
	return
}

func (cs *CollectionStore) AddCollectionRepos(ctx context.Context, crs []CollectionRepository) error {

	result, err := cs.db.Core.NewInsert().Model(&crs).Exec(ctx)
	if err != nil {
		return err
	}

	return assertAffectedXRows(int64(len(crs)), result, err)
}

func (cs *CollectionStore) RemoveCollectionRepos(ctx context.Context, crs []CollectionRepository) error {
	for _, cr := range crs {
		_, err := cs.db.Core.NewDelete().
			Model((*CollectionRepository)(nil)).
			Where("collection_id = ? AND repository_id = ?", cr.CollectionID, cr.RepositoryID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to remove repo %d from collection %d, error: %w", cr.RepositoryID, cr.CollectionID, err)
		}
	}
	return nil
}

func (cs *CollectionStore) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (collections []Collection, total int, err error) {
	query := cs.db.Operator.Core.
		NewSelect().
		Model(&collections).
		Relation("Repositories.Tags", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("category = ?", "task")
		}).
		Relation("Repositories", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Column(Fields...).OrderExpr("updated_at DESC").Limit(3)
		}).
		Where("collection.username = ?", username)

	if onlyPublic {
		query = query.Where("collection.private = ?", false)
	}
	query = query.Order("collection.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)
	err = query.Scan(ctx)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}
