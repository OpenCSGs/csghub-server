package database

import (
	"context"

	"github.com/uptrace/bun"
)

type OrgStore struct {
	db *DB
}

func NewOrgStore() *OrgStore {
	return &OrgStore{
		db: defaultDB,
	}
}

type Organization struct {
	ID          int64      `bun:",pk,autoincrement" json:"id"`
	Name        string     `bun:",notnull" json:"name"`
	Path        string     `bun:",notnull" json:"path"`
	GitPath     string     `bun:",notnull" json:"git_path"`
	Description string     `json:"description"`
	UserID      int64      `bun:",notnull" json:"user_id"`
	User        *User      `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceID int64      `bun:",pk" json:"namespace_id"`
	Namespace   *Namespace `bun:"rel:has-one,join:namespace_id=id" json:"namespace"`
	times
}

func (s *OrgStore) Create(ctx context.Context, org *Organization, namepace *Namespace) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(tx.NewInsert().Model(org).Exec(ctx)); err != nil {
			return err
		}
		namepace.NamespaceType = OrgNamespace
		if err = assertAffectedOneRow(tx.NewInsert().Model(namepace).Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	return
}

func (s *OrgStore) Index(ctx context.Context, username string) (orgs []Organization, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&orgs)
	if username != "" {
		query = query.
			Join("JOIN users AS u ON u.id = organization.user_id").
			Where("u.username =?", username)
	}

	err = query.Scan(ctx, &orgs)
	return
}

func (s *OrgStore) Update(ctx context.Context, org *Organization) (err error) {
	err = assertAffectedOneRow(s.db.Operator.Core.
		NewUpdate().
		Model(org).
		Exec(ctx))
	return
}

func (s *OrgStore) Delete(ctx context.Context, path string) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(
			tx.NewDelete().
				Model(&Organization{}).
				Where("path = ?", path).
				Exec(ctx)); err != nil {
			return err
		}
		if err = assertAffectedOneRow(
			tx.NewDelete().
				Model(&Namespace{}).
				Where("path = ?", path).
				Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	return
}

func (s *OrgStore) FindByName(ctx context.Context, path string) (org Organization, err error) {
	org.Name = path
	err = s.db.Operator.Core.
		NewSelect().
		Model(&org).
		Where("path =?", path).
		Scan(ctx)
	return
}
