package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type orgStoreImpl struct {
	db             *DB
	isHierarchical bool
}

type OrgStore interface {
	Create(ctx context.Context, org *Organization, namepace *Namespace) (err error)
	// CreateWithRelations atomically creates one legacy organization with its namespace, tags, and creator admin membership.
	CreateWithRelations(ctx context.Context, org *Organization, namepace *Namespace, tagIDs []int64) error
	GetUserOwnOrgs(ctx context.Context, username string) (orgs []Organization, total int, err error)
	IsLastOrganizationAdmin(ctx context.Context, username string) (bool, error)
	Update(ctx context.Context, org *Organization) (err error)
	Delete(ctx context.Context, path string) (err error)
	FindByPath(ctx context.Context, path string) (org Organization, err error)
	Exists(ctx context.Context, path string) (exists bool, err error)
	GetUserBelongOrgs(ctx context.Context, userID int64) (orgs []Organization, err error)
	GetUserRootOrganizations(ctx context.Context, userID int64) (orgs []Organization, err error)
	SearchUserBelongOrgs(ctx context.Context, userID int64, search string, per int, page int, orgType string, verifyStatus string, role string, tag string) (orgs []Organization, total int, err error)
	Search(ctx context.Context, search string, per, page int, orgType, verifyStatus, tag string) (orgs []Organization, total int, err error)
	UpdateVerifyStatus(ctx context.Context, path string, status types.VerifyStatus) error
	GetSharedOrgIDs(ctx context.Context, userIDs []int64) ([]int64, error)
	FindByUUID(ctx context.Context, uuid string) (*Organization, error)
	// Tag operations
	SetOrganizationTags(ctx context.Context, orgID int64, tagIDs []int64) error
	GetOrganizationTags(ctx context.Context, orgID int64) ([]Tag, error)
	GetOrganizationTagsByOrgIDs(ctx context.Context, orgIDs []int64) (map[int64][]Tag, error)
}

// NewOrgStore selects the legacy or hierarchy-aware organization Store according to configuration.
func NewOrgStore(cfg *config.Config) OrgStore {
	if cfg != nil && cfg.Organization.EnableUnit {
		return NewHierarchyOrgStoreWithDB(defaultDB)
	}
	return NewOrgStoreWithMode(defaultDB, false)
}

func NewOrgStoreWithDB(db *DB) OrgStore {
	return NewOrgStoreWithMode(db, false)
}

// NewOrgStoreWithMode creates an organization Store scoped to one organization model.
// The unit flag is persisted on organizations and keeps single-level and hierarchy queries separate.
func NewOrgStoreWithMode(db *DB, isHierarchical bool) OrgStore {
	return &orgStoreImpl{db: db, isHierarchical: isHierarchical}
}

type Organization struct {
	ID       int64  `bun:",pk,autoincrement" json:"id"`
	Nickname string `bun:"name,notnull" json:"name"`
	// unique name of the organization
	Name        string `bun:"path,notnull" json:"path"`
	GitPath     string `bun:",notnull" json:"git_path"`
	Description string `json:"description"`
	UserID      int64  `bun:",notnull" json:"user_id"`
	Homepage    string `bun:"" json:"homepage,omitempty"`
	Logo        string `bun:"" json:"logo,omitempty"`
	Verified    bool   `bun:"" json:"verified"`
	OrgType     string `bun:"" json:"org_type"`
	// IsRoot marks whether the organization is a top-level organization.
	IsRoot bool `bun:",notnull" json:"is_root"`
	// IsHierarchical marks whether the organization belongs to the hierarchy model.
	IsHierarchical bool               `bun:",notnull" json:"is_hierarchical"`
	User           *User              `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceID    int64              `bun:",notnull" json:"namespace_id"`
	Namespace      *Namespace         `bun:"rel:has-one,join:path=path" json:"namespace"`
	VerifyStatus   types.VerifyStatus `bun:",notnull,default:'none'" json:"verify_status"` // none, pending, approved, rejected
	UUID           uuid.UUID          `bun:"type:uuid,notnull,unique" json:"uuid"`
	Role           string             `bun:",scanonly" json:"role,omitempty"`
	// DeletedAt hides soft-deleted child organizations from normal queries.
	DeletedAt time.Time `bun:",soft_delete,nullzero" json:"deleted_at,omitempty"`
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
		org.IsRoot = true
		org.IsHierarchical = s.isHierarchical
		if err = s.createOrganizationTx(ctx, tx, org, namepace); err != nil {
			return err
		}
		return nil
	})
	err = errorx.HandleDBError(err, nil)
	return
}

// CreateWithRelations atomically creates an organization, its namespace, tags, and initial admin membership.
func (s *orgStoreImpl) CreateWithRelations(ctx context.Context, org *Organization, namepace *Namespace, tagIDs []int64) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		org.IsRoot = true
		org.IsHierarchical = s.isHierarchical
		if err := s.createOrganizationTx(ctx, tx, org, namepace); err != nil {
			return err
		}
		if err := s.setOrganizationTagsTx(ctx, tx, org.ID, tagIDs); err != nil {
			return err
		}
		if org.UserID != 0 {
			if err := s.addOrganizationMemberTx(ctx, tx, org.ID, org.UserID, string(types.UserAdmin)); err != nil {
				return err
			}
		}
		return nil
	})
	return errorx.HandleDBError(err, nil)
}

// createOrganizationTx inserts the organization and namespace rows and binds them together.
func (s *orgStoreImpl) createOrganizationTx(ctx context.Context, tx bun.Tx, org *Organization, namepace *Namespace) error {
	if err := assertAffectedOneRow(tx.NewInsert().Model(org).Exec(ctx)); err != nil {
		return err
	}
	namepace.NamespaceType = OrgNamespace
	err := assertAffectedOneRow(tx.NewInsert().Model(namepace).
		On("CONFLICT (path) WHERE deleted_at IS NULL DO UPDATE").
		Set("deleted_at = NULL, namespace_type = ?", namepace.NamespaceType).
		Exec(ctx))
	if err != nil {
		return err
	}
	org.NamespaceID = namepace.ID
	if err := assertAffectedOneRow(tx.NewUpdate().Model(org).WherePK().Exec(ctx)); err != nil {
		return err
	}
	return nil
}

// setOrganizationTagsTx replaces all organization tags within the caller's transaction.
func (s *orgStoreImpl) setOrganizationTagsTx(ctx context.Context, tx bun.Tx, orgID int64, tagIDs []int64) error {
	if _, err := tx.NewDelete().
		Model((*OrganizationTag)(nil)).
		Where("organization_id = ?", orgID).
		Exec(ctx); err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}
	orgTags := make([]OrganizationTag, len(tagIDs))
	for i, tagID := range tagIDs {
		orgTags[i] = OrganizationTag{
			OrganizationID: orgID,
			TagID:          tagID,
		}
	}
	if _, err := tx.NewInsert().Model(&orgTags).Exec(ctx); err != nil {
		return err
	}
	return nil
}

// addOrganizationMemberTx inserts one organization membership within the caller's transaction.
func (s *orgStoreImpl) addOrganizationMemberTx(ctx context.Context, tx bun.Tx, orgID, userID int64, role string) error {
	member := &Member{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
	}
	if _, err := tx.NewInsert().Model(member).Exec(ctx); err != nil {
		return err
	}
	return nil
}

func (s *orgStoreImpl) GetUserOwnOrgs(ctx context.Context, username string) (orgs []Organization, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&orgs).
		Relation("Namespace").
		Relation("User").
		Where("organization.is_hierarchical = ?", s.isHierarchical)
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

// IsLastOrganizationAdmin reports whether the user is the only active administrator of any organization.
// Organizations are evaluated independently, and only the admin role grants administrator status.
func (s *orgStoreImpl) IsLastOrganizationAdmin(ctx context.Context, username string) (bool, error) {
	if username == "" {
		return false, nil
	}
	exists, err := s.db.Operator.Core.NewSelect().
		Model((*Member)(nil)).
		Join("JOIN users AS target_user ON target_user.id = member.user_id AND target_user.deleted_at IS NULL").
		Join("JOIN organizations AS organization ON organization.id = member.organization_id AND organization.deleted_at IS NULL").
		Where("target_user.username = ?", username).
		Where("member.deleted_at IS NULL").
		Where("member.role = ?", types.UserAdmin).
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		Where(`NOT EXISTS (
			SELECT 1
			FROM members AS other_member
			JOIN users AS other_user
				ON other_user.id = other_member.user_id
				AND other_user.deleted_at IS NULL
			WHERE other_member.organization_id = member.organization_id
				AND other_member.user_id <> member.user_id
				AND other_member.deleted_at IS NULL
				AND other_member.role = ?
		)`, types.UserAdmin).
		Exists(ctx)
	return exists, errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) Update(ctx context.Context, org *Organization) (err error) {
	err = assertAffectedOneRow(s.db.Operator.Core.
		NewUpdate().
		Model(org).
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		WherePK().
		Exec(ctx))
	return errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) Delete(ctx context.Context, path string) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var org Organization
		org.Nickname = path
		if err = tx.NewSelect().Model(&org).Where("path = ? AND organization.is_hierarchical = ?", path, s.isHierarchical).Scan(ctx); err != nil {
			return err
		}
		// Clean up organization_tags
		if _, err = tx.NewDelete().
			Model((*OrganizationTag)(nil)).
			Where("organization_id = ?", org.ID).
			Exec(ctx); err != nil {
			return err
		}
		// Memberships have no database foreign key and must be removed in this transaction.
		if _, err = tx.NewDelete().
			Model((*Member)(nil)).
			Where("organization_id = ?", org.ID).
			ForceDelete().
			Exec(ctx); err != nil {
			return err
		}
		if err = assertAffectedOneRow(
			tx.NewDelete().
				Model(&Organization{}).
				Where("path = ?", path).
				ForceDelete().
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
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		Scan(ctx)
	return org, errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) Exists(ctx context.Context, path string) (exists bool, err error) {
	var org Organization
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&org).
		Where("path =?", path).
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		Exists(ctx)
	if err != nil {
		return exists, errorx.HandleDBError(err, nil)
	}
	return
}

// GetUserRootOrganizations returns active top-level organizations containing the user's membership.
func (s *orgStoreImpl) GetUserRootOrganizations(ctx context.Context, userID int64) (orgs []Organization, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&orgs).
		Relation("Namespace").
		ColumnExpr("organization.*").
		Where("organization.is_hierarchical = ? AND organization.is_root = TRUE", s.isHierarchical).
		Order("organization.id ASC")

	if s.isHierarchical {
		query = query.Where(`EXISTS (
			SELECT 1
			FROM members AS member
			INNER JOIN organization_units AS unit
				ON unit.organization_id = member.organization_id
				AND unit.root_organization_id = organization.id
				AND unit.deleted_at IS NULL
			WHERE member.user_id = ?
				AND member.deleted_at IS NULL
		)`, userID)
	} else {
		query = query.
			Join("INNER JOIN members ON members.organization_id = organization.id").
			Where("members.user_id = ? AND members.deleted_at IS NULL", userID)
	}

	err = query.Scan(ctx, &orgs)
	return orgs, errorx.HandleDBError(err, nil)
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
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		Scan(ctx, &orgs)
	return orgs, errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) SearchUserBelongOrgs(ctx context.Context, userID int64, search string, per int, page int, orgType string, verifyStatus string, role string, tag string) (orgs []Organization, total int, err error) {
	search = strings.ToLower(search)
	query := s.db.Operator.Core.NewSelect().
		Model(&orgs).Relation("Namespace")

	// Orgs the user belongs to as a member.
	query = query.
		ColumnExpr("organization.*").
		Join("join members on members.organization_id = organization.id").
		Where("members.user_id = ? and members.deleted_at is null", userID).
		Where("organization.is_hierarchical = ?", s.isHierarchical)
	switch types.UserRole(role) {
	case types.UserWrite:
		query = query.Where("members.role = ?", types.UserWrite)
	case types.UserAdmin:
		query = query.Where("members.role = ?", types.UserAdmin)
	default:
		// "all" or empty - no additional role filter.
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
	if tag != "" {
		query.Where(`EXISTS (
			SELECT 1 FROM organization_tags ot
			JOIN tags t ON t.id = ot.tag_id
			WHERE ot.organization_id = organization.id AND LOWER(t.name) = ?
		)`, strings.ToLower(tag))
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

func (s *orgStoreImpl) Search(ctx context.Context, search string, per int, page int, orgType, verifyStatus, tag string) (orgs []Organization, total int, err error) {
	search = strings.ToLower(search)
	query := s.db.Operator.Core.NewSelect().
		Model(&orgs).Relation("Namespace").
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		Where("organization.is_root = TRUE")
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
	if tag != "" {
		query.Where(`EXISTS (
			SELECT 1 FROM organization_tags ot
			JOIN tags t ON t.id = ot.tag_id
			WHERE ot.organization_id = organization.id AND LOWER(t.name) = ?
		)`, strings.ToLower(tag))
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
		Where("organization.is_hierarchical = ?", s.isHierarchical).
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
		Where("organization.is_hierarchical = ?", s.isHierarchical).
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
		Where("organization.is_hierarchical = ?", s.isHierarchical).
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
		return s.setOrganizationTagsTx(ctx, tx, orgID, tagIDs)
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
