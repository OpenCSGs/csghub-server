package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type orgStoreImpl struct {
	// rebacTupleJobClient records permission changes on this Store's business transaction.
	rebacTupleJobClient           RebacTupleJobClient
	db                            *DB
	isHierarchical                bool
	repositoryDeletionJobClient   RepositoryDeletionJobClient
	organizationDeletionJobClient OrganizationDeletionJobClient
}

// OrganizationDeletionJobClient inserts organization cleanup jobs in the same
// transaction that soft-deletes the organization records.
type OrganizationDeletionJobClient interface {
	InsertOrganizationDeletionJobTx(ctx context.Context, tx *sql.Tx, input OrganizationDeletionJobInput) (int64, error)
}

// OrganizationDeletionJobInput is the immutable cleanup snapshot consumed
// after an organization deletion transaction commits.
type OrganizationDeletionJobInput struct {
	OrganizationIDs               []int64
	OrganizationUUIDs             []string
	DeletedHierarchyRelationships []types.OrganizationHierarchyRelationship
	DeletedReBACRelationships     []types.OrganizationReBACCleanup
}

type OrgStore interface {
	// ReconcileNamespaceTuples queues ownership and any current creator admin role.
	ReconcileNamespaceTuples(ctx context.Context, orgID int64) error
	Create(ctx context.Context, org *Organization, namepace *Namespace) (err error)
	// CreateWithRelations atomically creates one legacy organization with its namespace, tags, and creator admin membership.
	CreateWithRelations(ctx context.Context, org *Organization, namepace *Namespace, tagIDs []int64) error
	GetUserOwnOrgs(ctx context.Context, username string) (orgs []Organization, total int, err error)
	IsLastOrganizationAdmin(ctx context.Context, username string) (bool, error)
	Update(ctx context.Context, org *Organization) (err error)
	Delete(ctx context.Context, path string) (OrganizationDeleteResult, error)
	// FindForDeletion resolves an organization and namespace including soft-deleted rows.
	FindForDeletion(ctx context.Context, lookup OrganizationDeletionLookup) (Organization, error)
	FindByPath(ctx context.Context, path string) (org Organization, err error)
	Exists(ctx context.Context, path string) (exists bool, err error)
	GetUserBelongOrgs(ctx context.Context, userID int64) (orgs []Organization, err error)
	GetUserRootOrganizations(ctx context.Context, userID int64) (orgs []Organization, err error)
	// SearchUserBelongOrgs searches organizations belonging to a user with filters and pagination.
	// Deprecated: this method only supports the deprecated user organization list endpoint.
	SearchUserBelongOrgs(ctx context.Context, userID int64, search string, per int, page int, orgType string, verifyStatus string, role string, tag string) (orgs []Organization, total int, err error)
	Search(ctx context.Context, search string, per, page int, orgType, verifyStatus, tag string) (orgs []Organization, total int, err error)
	// SearchHierarchyExcludingID searches hierarchy organizations by name or path without pagination.
	SearchHierarchyExcludingID(ctx context.Context, search string, excludedID int64, limit int) ([]Organization, error)
	// FindHierarchyByIDs returns hierarchy organizations and their parent names in one query.
	FindHierarchyByIDs(ctx context.Context, ids []int64) ([]Organization, error)
	UpdateVerifyStatus(ctx context.Context, path string, status types.VerifyStatus) error
	GetSharedOrgIDs(ctx context.Context, userIDs []int64) ([]int64, error)
	FindByUUID(ctx context.Context, uuid string) (*Organization, error)
	FindByUUIDs(ctx context.Context, uuids []string) ([]Organization, error)
	// Tag operations
	SetOrganizationTags(ctx context.Context, orgID int64, tagIDs []int64) error
	GetOrganizationTags(ctx context.Context, orgID int64) ([]Tag, error)
	GetOrganizationTagsByOrgIDs(ctx context.Context, orgIDs []int64) (map[int64][]Tag, error)
}

// NewOrgStore uses the initialized default database and the supplied organization mode.
// A nil jobClient is allowed unless repository deletion jobs need to be enqueued.
func NewOrgStore(isHierarchical bool, jobClient RepositoryDeletionJobClient) OrgStore {
	return NewOrgStoreWithJobClients(isHierarchical, jobClient, nil, nil)
}

// NewOrgStoreWithDeletionJobClients creates an organization store with both
// transactional repository and organization cleanup job clients.
func NewOrgStoreWithDeletionJobClients(isHierarchical bool, repositoryJobClient RepositoryDeletionJobClient, organizationJobClient OrganizationDeletionJobClient) OrgStore {
	return NewOrgStoreWithJobClients(isHierarchical, repositoryJobClient, organizationJobClient, nil)
}

// NewOrgStoreWithJobClients injects cleanup and permission job producers.
func NewOrgStoreWithJobClients(isHierarchical bool, repositoryJobs RepositoryDeletionJobClient, organizationJobs OrganizationDeletionJobClient, tupleJobs RebacTupleJobClient) OrgStore {
	return &orgStoreImpl{db: defaultDB, isHierarchical: isHierarchical, repositoryDeletionJobClient: repositoryJobs, organizationDeletionJobClient: organizationJobs, rebacTupleJobClient: tupleJobs}
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
	// ParentName is populated by hierarchy queries and is not persisted.
	ParentName string `bun:",scanonly" json:"-"`
	// DeletedAt retains deleted identities for cleanup retries and hides them from normal queries.
	DeletedAt time.Time `bun:",soft_delete,nullzero" json:"deleted_at,omitempty"`
	times
}

type OrganizationTag struct {
	ID             int64 `bun:",pk,autoincrement" json:"id"`
	OrganizationID int64 `bun:",notnull" json:"organization_id"`
	TagID          int64 `bun:",notnull" json:"tag_id"`
	times
}

// OrganizationDeleteResult contains post-commit cleanup metadata for a deleted legacy organization.
type OrganizationDeleteResult struct {
	DeletedRepositories       []DeletedRepository
	DeletedReBACRelationships []types.OrganizationReBACCleanup
	OrganizationJobID         int64
	AlreadyDeleted            bool
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
		if org.UserID != 0 {
			if err := lockUserRebacTx(ctx, tx, org.UserID); err != nil {
				return err
			}
		}
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
		return enqueueOrganizationCreationTx(ctx, tx, s.rebacTupleJobClient, *org, *namepace, org.UserID)
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

// Delete soft-deletes the organization and namespace and returns retained cleanup data.
func (s *orgStoreImpl) Delete(ctx context.Context, path string) (result OrganizationDeleteResult, err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var events []types.RebacTuple
		var org Organization
		org.Nickname = path
		if err = tx.NewSelect().Model(&org).WhereAllWithDeleted().
			Where("path = ? AND organization.is_hierarchical = ?", path, s.isHierarchical).
			OrderExpr("organization.deleted_at IS NULL DESC, organization.id DESC").Limit(1).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		alreadyDeleted := !org.DeletedAt.IsZero()
		var namespace Namespace
		namespaceQuery := tx.NewSelect().Model(&namespace).WhereAllWithDeleted().For("UPDATE")
		if org.NamespaceID != 0 {
			namespaceQuery.Where("id = ?", org.NamespaceID)
		} else {
			namespaceQuery.Where("path = ?", path)
		}
		if err = namespaceQuery.Scan(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// Repository deletion jobs are committed with the first soft deletion; retries only repeat external cleanup.
		if org.DeletedAt.IsZero() {
			repositoryIDs, err := findRepositoryIDsByNamespaces(ctx, tx, []string{path})
			if err != nil {
				return err
			}
			result.DeletedRepositories, err = deleteRepositoriesByIDs(ctx, tx, repositoryIDs, s.repositoryDeletionJobClient, &events)
			if err != nil {
				return err
			}
		}
		// Clean up organization_tags
		if _, err = tx.NewDelete().
			Model((*OrganizationTag)(nil)).
			Where("organization_id = ?", org.ID).
			Exec(ctx); err != nil {
			return err
		}
		// Retain memberships for authorization and ReBAC cleanup on retries.
		if _, err = tx.NewUpdate().Model((*Member)(nil)).
			Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").Where("organization_id = ? AND deleted_at IS NULL", org.ID).Exec(ctx); err != nil {
			return err
		}
		if _, err = tx.NewUpdate().Model((*Organization)(nil)).
			Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").Where("id = ? AND deleted_at IS NULL", org.ID).Exec(ctx); err != nil {
			return err
		}
		if namespace.ID != 0 && namespace.DeletedAt.IsZero() {
			if _, err = tx.NewUpdate().Model((*Namespace)(nil)).
				Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").Where("id = ? AND deleted_at IS NULL", namespace.ID).Exec(ctx); err != nil {
				return err
			}
		}
		result.DeletedReBACRelationships, err = loadOrganizationReBACCleanup(ctx, tx, []int64{org.ID})
		if err != nil {
			return err
		}
		if alreadyDeleted {
			result.AlreadyDeleted = true
			return nil
		}
		if s.organizationDeletionJobClient != nil {
			result.OrganizationJobID, err = s.organizationDeletionJobClient.InsertOrganizationDeletionJobTx(ctx, tx.Tx, OrganizationDeletionJobInput{
				OrganizationIDs:           []int64{org.ID},
				OrganizationUUIDs:         []string{org.UUID.String()},
				DeletedReBACRelationships: result.DeletedReBACRelationships,
			})
			if err != nil {
				return fmt.Errorf("enqueue organization deletion: %w", err)
			}
		}
		event, err := organizationDeletionTupleTx(ctx, tx, OrganizationDeletionJobInput{
			OrganizationIDs: []int64{org.ID}, DeletedReBACRelationships: result.DeletedReBACRelationships,
		})
		if err != nil {
			return err
		}
		events = append(events, event)
		return enqueueRebacTuple(ctx, tx, s.rebacTupleJobClient, events...)
	})
	if err != nil {
		return OrganizationDeleteResult{}, errorx.HandleDBError(err, nil)
	}
	return result, nil
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

// GetUserRootOrganizations returns active roots in the current mode for direct members.
// Hierarchy mode also includes roots reached through descendant memberships.
func (s *orgStoreImpl) GetUserRootOrganizations(ctx context.Context, userID int64) (orgs []Organization, err error) {
	if !s.isHierarchical {
		err := s.db.Operator.Core.NewSelect().
			Model(&orgs).
			Relation("Namespace").
			ColumnExpr("organization.*").
			ColumnExpr("member.role AS role").
			Join("JOIN members AS member ON member.organization_id = organization.id AND member.deleted_at IS NULL").
			Where("member.user_id = ?", userID).
			Where("organization.is_root = TRUE").
			Where("organization.is_hierarchical = FALSE").
			Where("organization.deleted_at IS NULL").
			Order("organization.id ASC").
			Scan(ctx, &orgs)
		return orgs, errorx.HandleDBError(err, nil)
	}

	query := s.db.Operator.Core.
		NewSelect().
		Model(&orgs).
		Relation("Namespace").
		ColumnExpr("organization.*").
		Where("organization.is_root = TRUE").
		Where("organization.is_hierarchical = TRUE").
		Where("organization.deleted_at IS NULL").
		Limit(1)
	query = query.WhereGroup("AND", func(q *bun.SelectQuery) *bun.SelectQuery {
		q = q.Where(`EXISTS (
		SELECT 1
		FROM members AS direct_member
		WHERE direct_member.organization_id = organization.id
			AND direct_member.user_id = ?
			AND direct_member.deleted_at IS NULL
	)`, userID)
		q = q.WhereOr(`EXISTS (
		SELECT 1
		FROM members AS hierarchy_member
		INNER JOIN organizations AS hierarchy_member_organization
			ON hierarchy_member_organization.id = hierarchy_member.organization_id
			AND hierarchy_member_organization.is_hierarchical = TRUE
			AND hierarchy_member_organization.deleted_at IS NULL
		INNER JOIN organization_units AS unit
			ON unit.organization_id = hierarchy_member.organization_id
			AND unit.root_organization_id = organization.id
			AND unit.deleted_at IS NULL
		WHERE hierarchy_member.user_id = ?
			AND hierarchy_member.deleted_at IS NULL
	)`, userID)
		return q
	})

	err = query.Scan(ctx, &orgs)
	return orgs, errorx.HandleDBError(err, nil)
}

func (s *orgStoreImpl) GetUserBelongOrgs(ctx context.Context, userID int64) (orgs []Organization, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&orgs).
		Relation("Namespace").
		ColumnExpr("organization.*").
		ColumnExpr("member.role AS role").
		Join("JOIN members AS member ON member.organization_id = organization.id AND member.deleted_at IS NULL").
		Where("member.user_id = ?", userID).
		Scan(ctx, &orgs)
	return orgs, errorx.HandleDBError(err, nil)
}

// SearchUserBelongOrgs searches active direct memberships in the current mode's root organizations.
// Deprecated: this method only supports the deprecated user organization list endpoint.
func (s *orgStoreImpl) SearchUserBelongOrgs(ctx context.Context, userID int64, search string, per int, page int, orgType string, verifyStatus string, role string, tag string) (orgs []Organization, total int, err error) {
	orgs = make([]Organization, 0)
	search = strings.ToLower(search)
	query := s.db.Operator.Core.NewSelect().
		Model(&orgs).Relation("Namespace")

	// Keep the legacy owner filter as an alias for the administrator role.
	if role == "owner" {
		role = string(types.UserAdmin)
	}
	query = query.
		ColumnExpr("organization.*").
		ColumnExpr("member.role AS role").
		Join("JOIN members AS member ON member.organization_id = organization.id AND member.deleted_at IS NULL").
		Where("member.user_id = ?", userID).
		Where("organization.is_root = TRUE").
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		Where("organization.deleted_at IS NULL")
	switch types.UserRole(role) {
	case types.UserWrite:
		query = query.Where("member.role = ?", types.UserWrite)
	case types.UserAdmin:
		query = query.Where("member.role = ?", types.UserAdmin)
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
	query.Order("organization.id ASC").Limit(per).Offset((page - 1) * per)
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
		Where("organization.is_root = TRUE").
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		Where("organization.deleted_at IS NULL")
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

// SearchHierarchyExcludingID searches active hierarchy organizations by name or path.
func (s *orgStoreImpl) SearchHierarchyExcludingID(ctx context.Context, search string, excludedID int64, limit int) (orgs []Organization, err error) {
	if limit <= 0 {
		limit = 20
	}
	pattern := fmt.Sprintf("%%%s%%", strings.ToLower(search))
	query := s.hierarchyQuery(&orgs).
		Where("(LOWER(organization.name) LIKE ? OR LOWER(organization.path) LIKE ?)", pattern, pattern).
		Where("organization.id <> ?", excludedID).
		Order("organization.id ASC").Limit(limit)
	err = query.Scan(ctx)
	return orgs, errorx.HandleDBError(err, nil)
}

// FindHierarchyByIDs returns active hierarchy organizations and their parent names in one query.
func (s *orgStoreImpl) FindHierarchyByIDs(ctx context.Context, ids []int64) (orgs []Organization, err error) {
	orgs = make([]Organization, 0)
	if len(ids) == 0 {
		return orgs, nil
	}
	err = s.hierarchyQuery(&orgs).
		Where("organization.id IN (?)", bun.In(ids)).
		Order("organization.id ASC").
		Scan(ctx)
	return orgs, errorx.HandleDBError(err, nil)
}

// hierarchyQuery builds the shared hierarchy organization projection used by search and batch lookups.
func (s *orgStoreImpl) hierarchyQuery(orgs *[]Organization) *bun.SelectQuery {
	return s.db.Operator.Core.NewSelect().
		Model(orgs).
		Relation("Namespace").
		ColumnExpr("organization.*").
		ColumnExpr("COALESCE(parent_organization.name, '') AS parent_name").
		Join("JOIN organization_units AS current_unit ON current_unit.organization_id = organization.id AND current_unit.deleted_at IS NULL").
		Join("LEFT JOIN organization_units AS parent_unit ON parent_unit.id = current_unit.parent_unit_id AND parent_unit.deleted_at IS NULL").
		Join("LEFT JOIN organizations AS parent_organization ON parent_organization.id = parent_unit.organization_id AND parent_organization.deleted_at IS NULL").
		Where("organization.is_hierarchical = TRUE")
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

// GetSharedOrgIDs returns active organizations that contain every user.
func (s *orgStoreImpl) GetSharedOrgIDs(ctx context.Context, userIDs []int64) ([]int64, error) {
	orgIDs := make([]int64, 0)
	if len(userIDs) == 0 {
		return orgIDs, nil
	}
	query := s.db.Operator.Core.NewSelect().
		Model(&Organization{}).
		Column("organization.id").
		Join("JOIN members AS member ON member.organization_id = organization.id AND member.deleted_at IS NULL").
		Where("member.user_id IN (?)", bun.In(userIDs)).
		Group("organization.id").
		Having("COUNT(DISTINCT member.user_id) = ?", len(userIDs))
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

// FindByUUIDs returns active organizations in the current mode. Legacy mode only exposes root organizations.
func (s *orgStoreImpl) FindByUUIDs(ctx context.Context, uuids []string) ([]Organization, error) {
	organizations := make([]Organization, 0)
	if len(uuids) == 0 {
		return organizations, nil
	}

	query := s.db.Operator.Core.NewSelect().
		Model(&organizations).
		Relation("Namespace").
		Where("organization.uuid IN (?)", bun.In(uuids)).
		Where("organization.is_hierarchical = ?", s.isHierarchical).
		Where("organization.deleted_at IS NULL")
	if !s.isHierarchical {
		// Legacy organizations are always top-level; hierarchy mode also includes child organizations.
		query = query.Where("organization.is_root = TRUE")
	}
	err := query.Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("uuids", uuids))
	}
	return organizations, nil
}

func (s *orgStoreImpl) SetOrganizationTags(ctx context.Context, orgID int64, tagIDs []int64) error {
	return s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return s.setOrganizationTagsTx(ctx, tx, orgID, tagIDs)
	})
}

// GetOrganizationTags returns organization tags in ID order and propagates query failures.
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

// OrganizationDeletionLookup selects a legacy path or an immutable hierarchy UUID.
type OrganizationDeletionLookup struct {
	Path string
	UUID string
}

// FindForDeletion includes tombstones only for deletion and binds the namespace by ID.
func (s *orgStoreImpl) FindForDeletion(ctx context.Context, lookup OrganizationDeletionLookup) (Organization, error) {
	var result Organization
	query := s.db.Core.NewSelect().Model(&result).WhereAllWithDeleted()
	if lookup.UUID != "" {
		query.Where("organization.uuid = ?", lookup.UUID)
	} else if lookup.Path != "" {
		query.Where("organization.path = ? AND organization.is_hierarchical = ?", lookup.Path, s.isHierarchical).
			OrderExpr("organization.deleted_at IS NULL DESC, organization.id DESC")
	} else {
		return Organization{}, errors.New("organization deletion identity is required")
	}
	if err := query.Limit(1).Scan(ctx); err != nil {
		return Organization{}, errorx.HandleDBError(err, nil)
	}
	org := &result
	var ns Namespace
	if org.NamespaceID != 0 {
		err := s.db.Core.NewSelect().Model(&ns).WhereAllWithDeleted().Where("id = ?", org.NamespaceID).Scan(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Organization{}, errorx.HandleDBError(err, nil)
		}
		if err == nil {
			org.Namespace = &ns
		}
	}

	return result, nil
}

// softDeleteOrganizationNamespaces also handles retries whose organization is already deleted.
func softDeleteOrganizationNamespaces(ctx context.Context, tx bun.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.NewUpdate().Model((*Namespace)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
		Where("id IN (SELECT namespace_id FROM organizations WHERE id IN (?)) AND deleted_at IS NULL", bun.In(ids)).Exec(ctx)
	return err
}

// namespaceTuple builds the owner relationship from committed identity fields.
func namespaceTuple(namespace Namespace, subjectUUID string) rebac.Relationship {
	relation := rebac.RelationOwner
	subject := rebac.UserSubject(subjectUUID)
	if namespace.NamespaceType == OrgNamespace {
		relation = rebac.RelationOrganization
		subject = rebac.NewSubject(rebac.ObjectTypeOrganization, subjectUUID)
	}
	return rebac.Relationship{Subject: subject, Relation: relation, Object: rebac.NamespaceObject(namespace.UUID)}
}

// enqueueOrganizationCreationTx snapshots namespace ownership and the real creator membership.
func enqueueOrganizationCreationTx(ctx context.Context, tx bun.Tx, tupleJobs RebacTupleJobClient, org Organization, namespace Namespace, creatorID int64) error {
	relationships := []rebac.Relationship{namespaceTuple(namespace, org.UUID.String())}
	if creatorID != 0 {
		var user User
		if err := tx.NewSelect().Model(&user).Column("uuid").Where("id = ?", creatorID).Scan(ctx); err != nil {
			return err
		}
		relationships = append(relationships, rebac.Relationship{Subject: rebac.UserSubject(user.UUID), Relation: rebac.RelationAdmin, Object: rebac.OrganizationObject(org.UUID.String())})
	}
	return enqueueRebacTuple(ctx, tx, tupleJobs, types.NewRebacTuple("organization.create", types.TupleOperation{Relationships: relationships}))
}

// ReconcileNamespaceTuples repairs only permissions supported by current DB rows.
func (s *orgStoreImpl) ReconcileNamespaceTuples(ctx context.Context, orgID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Resolve the creator before locking the organization to preserve lock order.
		var creatorID int64
		if err := tx.NewSelect().Model((*Organization)(nil)).Column("user_id").Where("id = ?", orgID).Scan(ctx, &creatorID); err != nil {
			return err
		}
		if creatorID != 0 {
			if err := lockUserRebacTx(ctx, tx, creatorID); err != nil {
				return err
			}
		}
		if err := lockTupleOrganization(ctx, tx, orgID); err != nil {
			return err
		}
		var org Organization
		if err := tx.NewSelect().Model(&org).Where("id = ?", orgID).Scan(ctx); err != nil {
			return err
		}
		if org.UserID != creatorID {
			return fmt.Errorf("organization creator changed during tuple reconciliation")
		}
		var ns Namespace
		if err := tx.NewSelect().Model(&ns).Where("id = ?", org.NamespaceID).Scan(ctx); err != nil {
			return err
		}
		var role types.UserRole
		err := tx.NewSelect().Model((*Member)(nil)).Column("role").Where("organization_id = ? AND user_id = ?", orgID, org.UserID).Scan(ctx, &role)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		creatorID = 0
		if role == types.UserAdmin {
			creatorID = org.UserID
		}
		return enqueueOrganizationCreationTx(ctx, tx, s.rebacTupleJobClient, org, ns, creatorID)
	})
}

// organizationDeletionTupleTx snapshots all direct relations, including queued writes.
// It reads grants without row locks; organization deletion already excludes new grants.
func organizationDeletionTupleTx(ctx context.Context, tx bun.Tx, input OrganizationDeletionJobInput) (types.RebacTuple, error) {
	var relationships []rebac.Relationship
	for _, edge := range input.DeletedHierarchyRelationships {
		relationships = append(relationships,
			rebac.Relationship{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, edge.ParentOrganizationUUID), Relation: rebac.RelationParent, Object: rebac.OrganizationObject(edge.ChildOrganizationUUID)},
			rebac.Relationship{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, edge.ChildOrganizationUUID), Relation: rebac.RelationChild, Object: rebac.OrganizationObject(edge.ParentOrganizationUUID)},
		)
	}
	for _, cleanup := range input.DeletedReBACRelationships {
		for _, userUUID := range cleanup.UserUUIDs {
			operations, err := memberTupleOperations(cleanup.OrganizationUUID, userUUID, "")
			if err != nil {
				return types.RebacTuple{}, err
			}
			relationships = append(relationships, operations[0].Relationships...)
		}
		if cleanup.NamespaceUUID != "" {
			relationships = append(relationships, namespaceTuple(Namespace{UUID: cleanup.NamespaceUUID, NamespaceType: OrgNamespace}, cleanup.OrganizationUUID))
		}
	}
	if len(input.OrganizationIDs) > 0 {
		var grants []RepositoryAuthorization
		if err := tx.NewSelect().Model(&grants).Where("subject_type = ? AND subject_id IN (?)", types.RepoAuthSubjectOrganization, bun.In(input.OrganizationIDs)).Order("id ASC").Scan(ctx); err != nil {
			return types.RebacTuple{}, err
		}
		for _, grant := range grants {
			r, err := authorizationTuple(grant)
			if err != nil {
				return types.RebacTuple{}, err
			}
			relationships = append(relationships, r)
		}
	}
	if len(relationships) == 0 {
		return types.RebacTuple{}, nil
	}
	return types.NewRebacTuple("organization.delete", types.TupleOperation{Delete: true, Relationships: relationships}), nil
}
