package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"

	"github.com/google/uuid"
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
	SearchUserBelongOrgs(ctx context.Context, userID int64, search string, per int, page int, orgType string, verifyStatus string, role string) (orgs []Organization, total int, err error)
	Search(ctx context.Context, search string, per, page int, orgType, verifyStatus string) (orgs []Organization, total int, err error)
	UpdateVerifyStatus(ctx context.Context, path string, status types.VerifyStatus) error
	GetSharedOrgIDs(ctx context.Context, userIDs []int64) ([]int64, error)
	FindByUUID(ctx context.Context, uuid string) (*Organization, error)
	// Tag operations
	SetOrganizationTags(ctx context.Context, orgID int64, tagIDs []int64) error
	GetOrganizationTags(ctx context.Context, orgID int64) ([]Tag, error)
	GetOrganizationTagsByOrgIDs(ctx context.Context, orgIDs []int64) (map[int64][]Tag, error)
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
	Name         string             `bun:"path,notnull" json:"path"`
	GitPath      string             `bun:",notnull" json:"git_path"`
	Description  string             `json:"description"`
	UserID       int64              `bun:",notnull" json:"user_id"`
	Homepage     string             `bun:"" json:"homepage,omitempty"`
	Logo         string             `bun:"" json:"logo,omitempty"`
	Verified     bool               `bun:"" json:"verified"`
	OrgType      string             `bun:"" json:"org_type"`
	User         *User              `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceID  int64              `bun:",notnull" json:"namespace_id"`
	Namespace    *Namespace         `bun:"rel:has-one,join:path=path" json:"namespace"`
	VerifyStatus types.VerifyStatus `bun:",notnull,default:'none'" json:"verify_status"` // none, pending, approved, rejected
	UUID         uuid.UUID          `bun:"type:uuid,notnull,unique" json:"uuid"`
	Role         string             `bun:",scanonly" json:"role,omitempty"`
	times
}

type OrganizationTag struct {
	ID             int64 `bun:",pk,autoincrement" json:"id"`
	OrganizationID int64 `bun:",notnull" json:"organization_id"`
	TagID          int64 `bun:",notnull" json:"tag_id"`
	times
}

func (s *orgStoreImpl) Create(ctx context.Context, org *Organization, namepace *Namespace) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(tx.NewInsert().Model(org).Exec(ctx)); err != nil {
			return err
		}
		namepace.NamespaceType = OrgNamespace
		err := assertAffectedOneRow(tx.NewInsert().Model(namepace).
			On("CONFLICT (path) DO UPDATE").
			Set("deleted_at = NULL, namespace_type = ?", namepace.NamespaceType).
			Exec(ctx))

		if err != nil {
			return err
		}
		// update org's namespace id
		org.NamespaceID = namepace.ID
		if err = assertAffectedOneRow(tx.NewUpdate().Model(org).WherePK().Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	err = errorx.HandleDBError(err, nil)
	return
}

func (s *orgStoreImpl) GetUserOwnOrgs(ctx context.Context, username string) (orgs []Organization, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&orgs).
		Relation("Namespace").
		Relation("User")
	if username != "" {
		query = query.
			Join("JOIN users AS u ON u.id = organization.user_id").
			Where("u.username =?", username)
	}

	err = query.Scan(ctx, &orgs)
	if err != nil {
		return orgs, total, errorx.HandleDBError(err, nil)
	}
	total = len(orgs)
	return
}

func (s *orgStoreImpl) Update(ctx context.Context, org *Organization) (err error) {
	err = assertAffectedOneRow(s.db.Operator.Core.
		NewUpdate().
		Model(org).
		WherePK().
		Exec(ctx))
	return errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) Delete(ctx context.Context, path string) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var org Organization
		org.Nickname = path
		if err = tx.NewSelect().Model(&org).Where("path = ?", path).Scan(ctx); err != nil {
			return err
		}
		// Clean up organization_tags
		if _, err = tx.NewDelete().
			Model((*OrganizationTag)(nil)).
			Where("organization_id = ?", org.ID).
			Exec(ctx); err != nil {
			return err
		}
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
				ForceDelete().
				Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	return errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) FindByPath(ctx context.Context, path string) (org Organization, err error) {
	org.Nickname = path
	err = s.db.Operator.Core.
		NewSelect().
		Model(&org).Relation("Namespace").
		Where("organization.path =?", path).
		Scan(ctx)
	return org, errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) Exists(ctx context.Context, path string) (exists bool, err error) {
	var org Organization
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&org).
		Where("path =?", path).
		Exists(ctx)
	if err != nil {
		return exists, errorx.HandleDBError(err, nil)
	}
	return
}

func (s *orgStoreImpl) GetUserBelongOrgs(ctx context.Context, userID int64) (orgs []Organization, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&orgs).
		Relation("Namespace").
		ColumnExpr("organization.*").
		ColumnExpr("members.role AS role").
		Join("join members on members.organization_id = organization.id").
		Where("members.user_id = ? and members.deleted_at is null", userID).
		Scan(ctx, &orgs)
	return orgs, errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) SearchUserBelongOrgs(ctx context.Context, userID int64, search string, per int, page int, orgType string, verifyStatus string, role string) (orgs []Organization, total int, err error) {
	search = strings.ToLower(search)
	query := s.db.Operator.Core.NewSelect().
		Model(&orgs).Relation("Namespace")

	if role == "owner" {
		// Orgs the user created/owns — filter by organization.user_id directly.
		query = query.Where("organization.user_id = ?", userID)
	} else {
		// Orgs the user belongs to as a member.
		query = query.
			ColumnExpr("organization.*").
			Join("join members on members.organization_id = organization.id").
			Where("members.user_id = ? and members.deleted_at is null", userID)
		switch role {
		case "write":
			query = query.Where("members.role = ?", "write")
		case "admin":
			query = query.Where("members.role = ?", "admin")
		default:
			// "all" or empty — no additional role filter.
		}
	}

	if search != "" {
		query.Where("LOWER(organization.name) like ? OR LOWER(organization.path) like ?", fmt.Sprintf("%%%s%%", search), fmt.Sprintf("%%%s%%", search))
		query.OrderExpr(`
			CASE
				WHEN LOWER(organization.path) = ? THEN 0
				WHEN LOWER(organization.path) LIKE ? THEN 1
				WHEN LOWER(organization.name) = ? THEN 2
				WHEN LOWER(organization.name) LIKE ? THEN 3
				ELSE 4
			END
		`, search, fmt.Sprintf("%s%%", search), search, fmt.Sprintf("%s%%", search))
	}
	if orgType != "" {
		query.Where("org_type = ?", orgType)
	}
	if verifyStatus != "" {
		query.Where("verify_status = ?", verifyStatus)
	}
	total, err = query.Count(ctx)
	if err != nil {
		return orgs, total, errorx.HandleDBError(err, nil)
	}
	query.Order("id asc").Limit(per).Offset((page - 1) * per)
	err = query.Scan(ctx, &orgs)
	if err != nil {
		return orgs, total, errorx.HandleDBError(err, nil)
	}
	return orgs, total, nil
}

func (s *orgStoreImpl) Search(ctx context.Context, search string, per int, page int, orgType, verifyStatus string) (orgs []Organization, total int, err error) {
	search = strings.ToLower(search)
	query := s.db.Operator.Core.NewSelect().
		Model(&orgs).Relation("Namespace")
	if search != "" {
		query.Where("LOWER(organization.name) like ? OR LOWER(organization.path) like ?", fmt.Sprintf("%%%s%%", search), fmt.Sprintf("%%%s%%", search))
		query.OrderExpr(`
			CASE
				WHEN LOWER(organization.path) = ? THEN 0
				WHEN LOWER(organization.path) LIKE ? THEN 1
				WHEN LOWER(organization.name) = ? THEN 2
				WHEN LOWER(organization.name) LIKE ? THEN 3
				ELSE 4
			END
		`, search, fmt.Sprintf("%s%%", search), search, fmt.Sprintf("%s%%", search))
	}
	if orgType != "" {
		query.Where("org_type = ?", orgType)
	}
	if verifyStatus != "" {
		query.Where("verify_status = ?", verifyStatus)
	}
	total, err = query.Count(ctx)
	if err != nil {
		return orgs, total, errorx.HandleDBError(err, nil)
	}
	query.Order("id asc").Limit(per).Offset((page - 1) * per)
	err = query.Scan(ctx, &orgs)
	if err != nil {
		return orgs, total, errorx.HandleDBError(err, nil)
	}
	return orgs, total, errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) UpdateVerifyStatus(ctx context.Context, path string, status types.VerifyStatus) error {
	_, err := s.db.Operator.Core.
		NewUpdate().
		Model(&Organization{}).
		Set("verify_status = ?", status).
		Where("path = ?", path).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}
	return nil
}

func (s *orgStoreImpl) GetSharedOrgIDs(ctx context.Context, userIDs []int64) ([]int64, error) {
	var orgIDs []int64
	if len(userIDs) == 0 {
		return orgIDs, nil
	}
	query := s.db.Operator.Core.NewSelect().
		Model(&Organization{}).
		Column("organization.id").
		Join("join members on members.organization_id = organization.id").
		Where("members.user_id IN (?)", bun.In(userIDs)).
		Group("organization.id").
		Having("COUNT(DISTINCT members.user_id) = ?", len(userIDs))
	err := query.Scan(ctx, &orgIDs)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return orgIDs, nil
}

func (s *orgStoreImpl) FindByUUID(ctx context.Context, uuid string) (*Organization, error) {
	var org Organization
	err := s.db.Operator.Core.NewSelect().
		Model(&org).Relation("Namespace").
		Where("organization.uuid = ?", uuid).
		Scan(ctx)
	if err == nil {
		return &org, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return nil, errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) SetOrganizationTags(ctx context.Context, orgID int64, tagIDs []int64) error {
	return s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Delete existing tags for this organization
		_, err := tx.NewDelete().
			Model((*OrganizationTag)(nil)).
			Where("organization_id = ?", orgID).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Insert new tags
		if len(tagIDs) > 0 {
			orgTags := make([]OrganizationTag, len(tagIDs))
			for i, tagID := range tagIDs {
				orgTags[i] = OrganizationTag{
					OrganizationID: orgID,
					TagID:          tagID,
				}
			}
			_, err = tx.NewInsert().Model(&orgTags).Exec(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *orgStoreImpl) GetOrganizationTags(ctx context.Context, orgID int64) ([]Tag, error) {
	var tags []Tag
	err := s.db.Operator.Core.NewSelect().
		Model(&tags).
		Join("JOIN organization_tags AS ot ON ot.tag_id = tag.id").
		Where("ot.organization_id = ?", orgID).
		Order("tag.id ASC").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return tags, nil
}

// tagWithOrgID is a helper struct for mapping tags to their organization.
type tagWithOrgID struct {
	Tag
	OrganizationID int64 `bun:"organization_id"`
}

// GetOrganizationTagsByOrgIDs returns a map of org ID to tags for the given org IDs.
func (s *orgStoreImpl) GetOrganizationTagsByOrgIDs(ctx context.Context, orgIDs []int64) (map[int64][]Tag, error) {
	if len(orgIDs) == 0 {
		return make(map[int64][]Tag), nil
	}
	var rows []tagWithOrgID
	err := s.db.Operator.Core.NewSelect().
		Model((*Tag)(nil)).
		ColumnExpr("tag.*").
		ColumnExpr("ot.organization_id").
		Join("JOIN organization_tags AS ot ON ot.tag_id = tag.id").
		Where("ot.organization_id IN (?)", bun.In(orgIDs)).
		Order("tag.id ASC").
		Scan(ctx, &rows)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	result := make(map[int64][]Tag, len(orgIDs))
	for _, row := range rows {
		result[row.OrganizationID] = append(result[row.OrganizationID], row.Tag)
	}
	return result, nil
}
