package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

type orgStoreImpl struct {
	db *DB
}

type OrgStore interface {
	Create(ctx context.Context, org *Organization, namepace *Namespace) (err error)
	GetUserOwnOrgs(ctx context.Context, username string) (orgs []Organization, total int, err error)
	Update(ctx context.Context, org *Organization) (err error)
	Delete(ctx context.Context, path string) (err error)
	FindByPath(ctx context.Context, path string) (org Organization, err error)
	Exists(ctx context.Context, path string) (exists bool, err error)
	GetUserBelongOrgs(ctx context.Context, userID int64) (orgs []Organization, err error)
	Search(ctx context.Context, search string, per, page int) (orgs []Organization, total int, err error)
}

func NewOrgStore() OrgStore {
	return &orgStoreImpl{
		db: defaultDB,
	}
}

func NewOrgStoreWithDB(db *DB) OrgStore {
	return &orgStoreImpl{
		db: db,
	}
}

type Organization struct {
	ID       int64  `bun:",pk,autoincrement" json:"id"`
	Nickname string `bun:"name,notnull" json:"name"`
	// unique name of the organization
	Name        string     `bun:"path,notnull" json:"path"`
	GitPath     string     `bun:",notnull" json:"git_path"`
	Description string     `json:"description"`
	UserID      int64      `bun:",notnull" json:"user_id"`
	Homepage    string     `bun:"" json:"homepage,omitempty"`
	Logo        string     `bun:"" json:"logo,omitempty"`
	Verified    bool       `bun:"" json:"verified"`
	OrgType     string     `bun:"" json:"org_type"`
	User        *User      `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceID int64      `bun:",notnull" json:"namespace_id"`
	Namespace   *Namespace `bun:"rel:has-one,join:namespace_id=id" json:"namespace"`
	times
}

func (s *orgStoreImpl) Create(ctx context.Context, org *Organization, namepace *Namespace) (err error) {
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

func (s *orgStoreImpl) GetUserOwnOrgs(ctx context.Context, username string) (orgs []Organization, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&orgs).
		Relation("User")
	if username != "" {
		query = query.
			Join("JOIN users AS u ON u.id = organization.user_id").
			Where("u.username =?", username)
	}

	err = query.Scan(ctx, &orgs)
	total = len(orgs)
	return
}

func (s *orgStoreImpl) Update(ctx context.Context, org *Organization) (err error) {
	err = assertAffectedOneRow(s.db.Operator.Core.
		NewUpdate().
		Model(org).
		WherePK().
		Exec(ctx))
	return
}

func (s *orgStoreImpl) Delete(ctx context.Context, path string) (err error) {
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

func (s *orgStoreImpl) FindByPath(ctx context.Context, path string) (org Organization, err error) {
	org.Nickname = path
	err = s.db.Operator.Core.
		NewSelect().
		Model(&org).
		Where("path =?", path).
		Scan(ctx)
	return
}

func (s *orgStoreImpl) Exists(ctx context.Context, path string) (exists bool, err error) {
	var org Organization
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&org).
		Where("path =?", path).
		Exists(ctx)
	if err != nil {
		return
	}
	return
}

func (s *orgStoreImpl) GetUserBelongOrgs(ctx context.Context, userID int64) (orgs []Organization, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&orgs).
		Join("join members on members.organization_id = organization.id").
		Where("members.user_id = ?", userID).
		Scan(ctx, &orgs)
	return
}

func (s *orgStoreImpl) Search(ctx context.Context, search string, per int, page int) (orgs []Organization, total int, err error) {
	search = strings.ToLower(search)
	query := s.db.Operator.Core.NewSelect().
		Model(&orgs)
	if search != "" {
		query.Where("LOWER(name) like ? OR LOWER(path) like ?", fmt.Sprintf("%%%s%%", search), fmt.Sprintf("%%%s%%", search))
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	query.Order("id asc").Limit(per).Offset((page - 1) * per)
	err = query.Scan(ctx, &orgs)
	if err != nil {
		return
	}
	return
}
