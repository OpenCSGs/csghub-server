package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// OrganizationUnit stores only the structural position of a real organization in a hierarchy.
type OrganizationUnit struct {
	bun.BaseModel `bun:"table:organization_units,alias:ou"`

	ID                 int64      `bun:",pk,autoincrement"`
	RootOrganizationID int64      `bun:",notnull"`
	OrganizationID     int64      `bun:",notnull"`
	ParentUnitID       *int64     `bun:",nullzero"`
	SortOrder          int        `bun:",notnull"`
	CreatedAt          time.Time  `bun:",nullzero,notnull,skipupdate,default:current_timestamp"`
	UpdatedAt          time.Time  `bun:",nullzero,notnull,default:current_timestamp"`
	DeletedAt          *time.Time `bun:",soft_delete,nullzero"`

	RootOrganizationUUID string  `bun:"root_organization_uuid,scanonly"`
	OrganizationUUID     string  `bun:"organization_uuid,scanonly"`
	ParentUnitUUID       *string `bun:"parent_unit_uuid,scanonly"`
	Name                 string  `bun:"name,scanonly"`
	Nickname             string  `bun:"nickname,scanonly"`
	Description          string  `bun:"description,scanonly"`
	Homepage             string  `bun:"homepage,scanonly"`
	Logo                 string  `bun:"logo,scanonly"`
	OrgType              string  `bun:"org_type,scanonly"`
	Verified             bool    `bun:"verified,scanonly"`
	UserID               int64   `bun:"user_id,scanonly"`
	NamespaceUUID        string  `bun:"namespace_uuid,scanonly"`
	Depth                int     `bun:"depth,scanonly"`
	DirectChildrenCount  int     `bun:"direct_children_count,scanonly"`
	SubtreeMemberCount   int     `bun:"subtree_member_count,scanonly"`
}

// OrganizationUnitClosure stores derived ancestor-to-descendant reachability inside one root hierarchy.
type OrganizationUnitClosure struct {
	bun.BaseModel `bun:"table:organization_unit_closure,alias:ouc"`

	RootOrganizationID int64     `bun:",pk"`
	AncestorUnitID     int64     `bun:",pk"`
	DescendantUnitID   int64     `bun:",pk"`
	Depth              int       `bun:",notnull"`
	CreatedAt          time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

// OrganizationUnitStore persists hierarchy organizations and their relationships atomically.
type OrganizationUnitStore interface {
	CreateRoot(ctx context.Context, input CreateRootOrganizationInput) (*Organization, error)
	DeleteRoot(ctx context.Context, input DeleteRootOrganizationInput) (*types.DeleteRootOrganizationResp, error)
	Create(ctx context.Context, input CreateOrganizationUnitInput) (*types.OrganizationUnit, error)
	FindByUUID(ctx context.Context, unitUUID string) (*OrganizationUnit, error)
	Update(ctx context.Context, input UpdateOrganizationUnitInput) (*types.OrganizationUnit, error)
	Delete(ctx context.Context, input DeleteOrganizationUnitInput) (*types.DeleteOrganizationUnitResp, error)
	ListRoots(ctx context.Context, input ListOrganizationUnitInput) ([]types.OrganizationUnitSummary, int, error)
	ListChildren(ctx context.Context, input ListOrganizationUnitInput) ([]types.OrganizationUnitSummary, int, error)
}

// CreateRootOrganizationInput contains the root organization records and creator membership.
type CreateRootOrganizationInput struct {
	Organization  *Organization
	Namespace     *Namespace
	TagIDs        []int64
	CreatorUserID int64
}

// DeleteRootOrganizationInput identifies a top-level organization hierarchy.
type DeleteRootOrganizationInput struct {
	OrganizationUUID string
}

// CreateOrganizationUnitInput contains a validated child organization and its structural placement.
type CreateOrganizationUnitInput struct {
	RootOrganizationID int64
	ParentUnitUUID     *string
	Organization       *Organization
	Namespace          *Namespace
	TagIDs             []int64
	SortOrder          int
	MaxDepth           int
}

// UpdateOrganizationUnitInput contains mutable organization metadata and structural fields for a root or child organization.
type UpdateOrganizationUnitInput struct {
	RootOrganizationID int64
	OrganizationID     int64
	UnitID             int64
	UnitUUID           string
	Nickname           *string
	Description        *string
	Homepage           *string
	Logo               *string
	Verified           *bool
	OrgType            *string
	TagIDs             *[]int64
	SortOrder          *int
}

// DeleteOrganizationUnitInput identifies a child organization subtree.
type DeleteOrganizationUnitInput struct {
	RootOrganizationID int64
	UnitID             int64
	UnitUUID           string
}

type organizationRepositoryNamespace struct {
	ID               int64  `bun:"id"`
	Path             string `bun:"path"`
	OrganizationUUID string `bun:"organization_uuid"`
}

// ListOrganizationUnitInput contains one-level hierarchy filters and pagination.
type ListOrganizationUnitInput struct {
	RootOrganizationID int64
	UnitID             int64
	Per                int
	Page               int
}

// organizationUnitStoreImpl uses one database connection for all hierarchy transactions.
type organizationUnitStoreImpl struct {
	db                          *DB
	repositoryDeletionJobClient RepositoryDeletionJobClient
}

// NewOrganizationUnitStore creates an organization hierarchy Store.
func NewOrganizationUnitStore() OrganizationUnitStore {
	return NewOrganizationUnitStoreWithDB(GetDB())
}

// NewOrganizationUnitStoreWithDB creates a hierarchy Store with an explicit database.
func NewOrganizationUnitStoreWithDB(db *DB) OrganizationUnitStore {
	return NewOrganizationUnitStoreWithDBAndDeletionJobClient(db, nil)
}

// NewOrganizationUnitStoreWithDBAndDeletionJobClient creates a hierarchy Store with transactional repository deletion jobs.
func NewOrganizationUnitStoreWithDBAndDeletionJobClient(db *DB, jobClient RepositoryDeletionJobClient) OrganizationUnitStore {
	return &organizationUnitStoreImpl{db: db, repositoryDeletionJobClient: jobClient}
}

// CreateRoot atomically creates a top-level organization, its root unit, closure self-row, and admin member.
func (s *organizationUnitStoreImpl) CreateRoot(ctx context.Context, input CreateRootOrganizationInput) (*Organization, error) {
	if input.Organization == nil || input.Namespace == nil {
		return nil, errorx.ReqParamInvalid(errors.New("root organization and namespace are required"), nil)
	}
	input.Organization.IsRoot = true
	input.Organization.IsHierarchical = true
	input.Namespace.NamespaceType = OrgNamespace
	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(input.Organization).Exec(ctx); err != nil {
			return fmt.Errorf("create root organization: %w", err)
		}
		if _, err := tx.NewInsert().Model(input.Namespace).Exec(ctx); err != nil {
			return fmt.Errorf("create root organization namespace: %w", err)
		}
		input.Organization.NamespaceID = input.Namespace.ID
		if _, err := tx.NewUpdate().Model(input.Organization).Column("namespace_id").WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("link root organization namespace: %w", err)
		}
		if err := insertOrganizationTags(ctx, tx, input.Organization.ID, input.TagIDs); err != nil {
			return err
		}
		unit := &OrganizationUnit{
			RootOrganizationID: input.Organization.ID,
			OrganizationID:     input.Organization.ID,
		}
		if _, err := tx.NewInsert().Model(unit).Exec(ctx); err != nil {
			return fmt.Errorf("create root organization unit: %w", err)
		}
		if input.CreatorUserID != 0 {
			if err := upsertOrganizationMember(ctx, tx, input.Organization.ID, input.CreatorUserID, types.UserAdmin); err != nil {
				return fmt.Errorf("create root organization administrator membership: %w", err)
			}
		}
		if _, err := tx.NewInsert().Model(&OrganizationUnitClosure{
			RootOrganizationID: input.Organization.ID,
			AncestorUnitID:     unit.ID,
			DescendantUnitID:   unit.ID,
			Depth:              0,
		}).Exec(ctx); err != nil {
			return fmt.Errorf("create root organization closure: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	var organization Organization
	if err := s.db.Core.NewSelect().Model(&organization).
		Relation("Namespace").
		Where("organization.uuid = ?", input.Organization.UUID).
		Scan(ctx); err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &organization, nil
}

// DeleteRoot atomically soft-deletes a top-level organization and its complete hierarchy.
func (s *organizationUnitStoreImpl) DeleteRoot(ctx context.Context, input DeleteRootOrganizationInput) (*types.DeleteRootOrganizationResp, error) {
	result := &types.DeleteRootOrganizationResp{}
	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var root Organization
		if err := tx.NewSelect().Model(&root).WhereAllWithDeleted().
			Where("organization.uuid = ? AND organization.is_hierarchical = TRUE", input.OrganizationUUID).
			For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock root organization: %w", err)
		}
		if !root.IsRoot {
			return errorx.ReqParamInvalid(errors.New("organization_uuid must identify a top-level organization"), nil)
		}

		allOrganizationIDs, err := loadHierarchyOrganizationIDs(ctx, tx, root.ID, true)
		if err != nil {
			return err
		}
		if err := tx.NewSelect().Model((*Organization)(nil)).
			ColumnExpr("CAST(organization.uuid AS TEXT)").
			WhereAllWithDeleted().
			Where("id IN (?) AND is_hierarchical = TRUE", bun.In(allOrganizationIDs)).
			Order("id ASC").Scan(ctx, &result.DeletedOrganizationUUIDs); err != nil {
			return errorx.HandleDBError(err, nil)
		}
		result.DeletedOrganizationIDs = append(result.DeletedOrganizationIDs, allOrganizationIDs...)
		result.DeletedHierarchyRelationships, err = loadOrganizationHierarchyRelationships(ctx, tx, root.ID, nil, !root.DeletedAt.IsZero())
		if err != nil {
			return fmt.Errorf("load hierarchy relationships for ReBAC cleanup: %w", err)
		}
		result.DeletedReBACRelationships, err = loadOrganizationReBACCleanup(ctx, tx, allOrganizationIDs)
		if err != nil {
			return fmt.Errorf("load hierarchy member and namespace relationships for ReBAC cleanup: %w", err)
		}
		if !root.DeletedAt.IsZero() {
			result.AlreadyDeleted = true
			return nil
		}

		activeUnits := make([]OrganizationUnit, 0)
		if err := tx.NewSelect().Model(&activeUnits).
			Column("id", "organization_id").
			Where("root_organization_id = ?", root.ID).
			Order("id ASC").Scan(ctx); err != nil {
			return fmt.Errorf("load root organization units: %w", err)
		}
		activeOrganizationIDs := uniqueOrganizationIDs(root.ID, activeUnits)
		var namespaces []organizationRepositoryNamespace
		if err := tx.NewSelect().Model((*Organization)(nil)).
			ColumnExpr("organization.namespace_id AS id").ColumnExpr("namespace.path AS path").
			ColumnExpr("CAST(organization.uuid AS TEXT) AS organization_uuid").
			Join("JOIN namespaces AS namespace ON namespace.id = organization.namespace_id").
			Where("organization.id IN (?) AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL", bun.In(activeOrganizationIDs)).
			OrderExpr("namespace.path ASC").Scan(ctx, &namespaces); err != nil {
			return fmt.Errorf("load hierarchy organization namespaces: %w", err)
		}
		namespaceIDs, namespacePaths := organizationNamespaceIDsAndPaths(namespaces)
		if err := lockOrganizationRepositoryNamespaces(ctx, tx, namespacePaths); err != nil {
			return err
		}
		repositoryIDs, err := findRepositoryIDsByNamespaces(ctx, tx, namespacePaths)
		if err != nil {
			return err
		}
		deletedRepositories, err := deleteRepositoriesByIDs(ctx, tx, repositoryIDs, s.repositoryDeletionJobClient)
		if err != nil {
			return err
		}
		result.DeletedRepositories = organizationDeletedRepositories(deletedRepositories, namespaces)
		if err := tx.NewSelect().Model((*Member)(nil)).ColumnExpr("COUNT(DISTINCT member.user_id)").
			Where("member.organization_id IN (?) AND member.deleted_at IS NULL", bun.In(allOrganizationIDs)).
			Scan(ctx, &result.UsersAffected); err != nil {
			return fmt.Errorf("count hierarchy organization members: %w", err)
		}
		memberResult, err := tx.NewUpdate().Model((*Member)(nil)).
			Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("member.organization_id IN (?) AND member.deleted_at IS NULL", bun.In(allOrganizationIDs)).Exec(ctx)
		if err != nil {
			return fmt.Errorf("remove hierarchy organization members: %w", err)
		}
		if affected, err := memberResult.RowsAffected(); err == nil {
			result.MembersRemoved = int(affected)
		}
		if _, err := tx.NewDelete().Model((*OrganizationUnitClosure)(nil)).
			Where("root_organization_id = ?", root.ID).Exec(ctx); err != nil {
			return fmt.Errorf("delete root organization closure: %w", err)
		}
		unitResult, err := tx.NewUpdate().Model((*OrganizationUnit)(nil)).
			Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("ou.root_organization_id = ? AND ou.deleted_at IS NULL", root.ID).Exec(ctx)
		if err != nil {
			return fmt.Errorf("soft-delete root organization units: %w", err)
		}
		if affected, err := unitResult.RowsAffected(); err == nil {
			result.UnitsDeleted = int(affected)
		}
		organizationResult, err := tx.NewUpdate().Model((*Organization)(nil)).
			Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("organization.id IN (?) AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL", bun.In(activeOrganizationIDs)).Exec(ctx)
		if err != nil {
			return fmt.Errorf("soft-delete hierarchy organizations: %w", err)
		}
		if affected, err := organizationResult.RowsAffected(); err == nil {
			result.OrganizationsDeleted = int(affected)
		}
		if len(namespaceIDs) > 0 {
			if _, err := tx.NewUpdate().Model((*Namespace)(nil)).
				Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
				Where("namespace.id IN (?) AND namespace.deleted_at IS NULL", bun.In(namespaceIDs)).Exec(ctx); err != nil {
				return fmt.Errorf("soft-delete hierarchy organization namespaces: %w", err)
			}
		}
		if _, err := tx.NewDelete().Model((*OrganizationTag)(nil)).
			Where("organization_id IN (?)", bun.In(allOrganizationIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("delete hierarchy organization tags: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

// loadHierarchyOrganizationIDs returns every real organization ever attached to a root hierarchy.
func loadHierarchyOrganizationIDs(ctx context.Context, tx bun.Tx, rootOrganizationID int64, includeDeleted bool) ([]int64, error) {
	units := make([]OrganizationUnit, 0)
	query := tx.NewSelect().Model(&units).Column("organization_id").Where("root_organization_id = ?", rootOrganizationID)
	if includeDeleted {
		query = query.WhereAllWithDeleted()
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("load hierarchy organization IDs: %w", err)
	}
	return uniqueOrganizationIDs(rootOrganizationID, units), nil
}

// uniqueOrganizationIDs builds a stable, duplicate-free organization ID list including the root.
func uniqueOrganizationIDs(rootOrganizationID int64, units []OrganizationUnit) []int64 {
	seen := map[int64]struct{}{rootOrganizationID: {}}
	organizationIDs := []int64{rootOrganizationID}
	for _, unit := range units {
		if _, exists := seen[unit.OrganizationID]; exists {
			continue
		}
		seen[unit.OrganizationID] = struct{}{}
		organizationIDs = append(organizationIDs, unit.OrganizationID)
	}
	return organizationIDs
}

func lockOrganizationRepositoryNamespaces(ctx context.Context, tx bun.Tx, namespacePaths []string) error {
	paths := canonicalNamespaceLockOrder(namespacePaths)
	for _, path := range paths {
		var namespace Namespace
		query := tx.NewSelect().Model(&namespace).Where("path = ? AND deleted_at IS NULL", path).Limit(1)
		if tx.Dialect().Name() == dialect.PG {
			query.For("UPDATE")
		}
		if err := query.Scan(ctx); err != nil {
			return fmt.Errorf("lock repository namespace %q for deletion: %w", path, err)
		}
	}
	return nil
}

func organizationDeletedRepositories(repositories []DeletedRepository, namespaces []organizationRepositoryNamespace) []types.DeletedRepository {
	organizationUUIDByPath := make(map[string]string, len(namespaces))
	for _, namespace := range namespaces {
		organizationUUIDByPath[namespace.Path] = namespace.OrganizationUUID
	}
	result := make([]types.DeletedRepository, 0, len(repositories))
	for _, repository := range repositories {
		namespacePath, _ := Repository{Path: repository.Path}.NamespaceAndName()
		result = append(result, types.DeletedRepository{
			ID: repository.ID, RepositoryType: repository.RepositoryType, Path: repository.Path,
			OrganizationUUID: organizationUUIDByPath[namespacePath],
		})
	}
	return result
}

func organizationNamespaceIDsAndPaths(namespaces []organizationRepositoryNamespace) ([]int64, []string) {
	ids := make([]int64, 0, len(namespaces))
	paths := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		ids = append(ids, namespace.ID)
		paths = append(paths, namespace.Path)
	}
	return ids, paths
}

// lockOrganization serializes hierarchy writes for one organization.
func lockOrganization(ctx context.Context, tx bun.Tx, organizationID int64) error {
	var id int64
	err := tx.NewSelect().Model((*Organization)(nil)).Column("id").
		Where("organization.id = ? AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL", organizationID).
		For("UPDATE").Scan(ctx, &id)
	if err != nil {
		return fmt.Errorf("lock organization %d: %w", organizationID, err)
	}
	return nil
}

// selectUnitForUpdate locks one active unit inside its root hierarchy.
func selectUnitForUpdate(ctx context.Context, tx bun.Tx, rootOrganizationID, unitID int64) (*OrganizationUnit, error) {
	var unit OrganizationUnit
	err := tx.NewSelect().Model(&unit).
		Where("ou.id = ? AND ou.root_organization_id = ? AND ou.deleted_at IS NULL", unitID, rootOrganizationID).
		For("UPDATE").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock organization unit: %w", err)
	}
	return &unit, nil
}

// selectUnitByUUID resolves a unit by the UUID of its real organization.
func selectUnitByUUID(ctx context.Context, db bun.IDB, unitUUID string) (*OrganizationUnit, error) {
	var unit OrganizationUnit
	err := unitSelectQuery(db, &unit).
		Where("organization.uuid = ?", unitUUID).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("unit_uuid", unitUUID))
	}
	return &unit, nil
}

// unitSelectQuery adds the organization-backed public fields shared by unit queries.
func unitSelectQuery(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).
		Column("ou.*").
		ColumnExpr("root_organization.uuid::text AS root_organization_uuid").
		ColumnExpr("organization.uuid::text AS organization_uuid").
		ColumnExpr("parent_organization.uuid::text AS parent_unit_uuid").
		ColumnExpr("organization.path AS name").
		ColumnExpr("organization.name AS nickname").
		ColumnExpr("organization.description").
		ColumnExpr("organization.homepage").
		ColumnExpr("organization.logo").
		ColumnExpr("organization.org_type").
		ColumnExpr("organization.verified").
		ColumnExpr("organization.user_id").
		ColumnExpr("COALESCE(namespace.uuid, '') AS namespace_uuid").
		ColumnExpr("COALESCE((SELECT MAX(c.depth) FROM organization_unit_closure AS c WHERE c.root_organization_id = ou.root_organization_id AND c.descendant_unit_id = ou.id), 0) AS depth").
		Join("JOIN organizations AS organization ON organization.id = ou.organization_id AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL").
		Join("JOIN organizations AS root_organization ON root_organization.id = ou.root_organization_id AND root_organization.is_hierarchical = TRUE AND root_organization.deleted_at IS NULL").
		Join("LEFT JOIN organization_units AS parent ON parent.id = ou.parent_unit_id AND parent.deleted_at IS NULL").
		Join("LEFT JOIN organizations AS parent_organization ON parent_organization.id = parent.organization_id AND parent_organization.is_hierarchical = TRUE AND parent_organization.deleted_at IS NULL").
		Join("LEFT JOIN namespaces AS namespace ON namespace.id = organization.namespace_id AND namespace.deleted_at IS NULL")
}

// toOrganizationUnit maps a database record to the public organization-backed unit type.
func toOrganizationUnit(unit *OrganizationUnit) types.OrganizationUnit {
	if unit == nil {
		return types.OrganizationUnit{}
	}
	return types.OrganizationUnit{
		OrganizationID:       unit.OrganizationID,
		UUID:                 unit.OrganizationUUID,
		RootOrganizationUUID: unit.RootOrganizationUUID,
		OrganizationUUID:     unit.OrganizationUUID,
		ParentUnitUUID:       unit.ParentUnitUUID,
		Name:                 unit.Name,
		Nickname:             unit.Nickname,
		Description:          unit.Description,
		Homepage:             unit.Homepage,
		Logo:                 unit.Logo,
		OrgType:              unit.OrgType,
		Verified:             unit.Verified,
		UserID:               unit.UserID,
		IsRoot:               unit.OrganizationID == unit.RootOrganizationID,
		IsHierarchical:       true,
		Namespace:            &types.Namespace{Path: unit.Name, Type: string(OrgNamespace), UUID: unit.NamespaceUUID},
		SortOrder:            unit.SortOrder,
		Depth:                unit.Depth,
		CreatedAt:            unit.CreatedAt,
		UpdatedAt:            unit.UpdatedAt,
		DeletedAt:            unit.DeletedAt,
		Tags:                 make([]types.RepoTag, 0),
	}
}

// toOrganizationUnitSummary adds direct-child and subtree-member counters.
func toOrganizationUnitSummary(unit *OrganizationUnit) types.OrganizationUnitSummary {
	return types.OrganizationUnitSummary{
		OrganizationUnit:    toOrganizationUnit(unit),
		DirectChildrenCount: unit.DirectChildrenCount,
		SubtreeMemberCount:  unit.SubtreeMemberCount,
	}
}

// FindByUUID returns an active unit by its real organization UUID.
func (s *organizationUnitStoreImpl) FindByUUID(ctx context.Context, unitUUID string) (*OrganizationUnit, error) {
	return selectUnitByUUID(ctx, s.db.Core, unitUUID)
}

// Create atomically creates a child organization, namespace, unit, closure, and tags.
func (s *organizationUnitStoreImpl) Create(ctx context.Context, input CreateOrganizationUnitInput) (*types.OrganizationUnit, error) {
	if input.Organization == nil || input.Namespace == nil {
		return nil, errorx.ReqParamInvalid(errors.New("child organization and namespace are required"), nil)
	}
	unit := &OrganizationUnit{RootOrganizationID: input.RootOrganizationID, SortOrder: input.SortOrder}
	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		var rootIsRoot bool
		if err := tx.NewSelect().Model((*Organization)(nil)).Column("is_root").
			Where("organization.id = ? AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL", input.RootOrganizationID).
			Scan(ctx, &rootIsRoot); err != nil {
			return fmt.Errorf("load root organization: %w", err)
		}
		if !rootIsRoot {
			return errorx.ReqParamInvalid(errors.New("root organization UUID does not identify a top-level organization"), nil)
		}

		if input.ParentUnitUUID != nil {
			var parent OrganizationUnit
			if err := tx.NewSelect().Model(&parent).
				Join("JOIN organizations AS parent_organization ON parent_organization.id = ou.organization_id AND parent_organization.is_hierarchical = TRUE AND parent_organization.deleted_at IS NULL").
				Where("parent_organization.uuid = ? AND ou.root_organization_id = ?", *input.ParentUnitUUID, input.RootOrganizationID).
				For("UPDATE").Scan(ctx); err != nil {
				return fmt.Errorf("find parent organization unit: %w", err)
			}
			var parentDepth int
			if err := tx.NewSelect().Model((*OrganizationUnitClosure)(nil)).
				ColumnExpr("COALESCE(MAX(depth), 0)").
				Where("root_organization_id = ? AND descendant_unit_id = ?", input.RootOrganizationID, parent.ID).
				Scan(ctx, &parentDepth); err != nil {
				return fmt.Errorf("get parent depth: %w", err)
			}
			if parentDepth+1 >= input.MaxDepth {
				return errorx.ReqParamInvalid(fmt.Errorf("organization unit maximum depth %d exceeded", input.MaxDepth), nil)
			}
			unit.ParentUnitID = &parent.ID
		} else {
			// New root organizations have a structural root unit. Attach the first
			// real child level to it while preserving compatibility for older roots.
			var rootUnitID int64
			err := tx.NewSelect().Model((*OrganizationUnit)(nil)).Column("id").
				Where("ou.root_organization_id = ? AND ou.organization_id = ? AND ou.parent_unit_id IS NULL AND ou.deleted_at IS NULL", input.RootOrganizationID, input.RootOrganizationID).
				Scan(ctx, &rootUnitID)
			switch {
			case err == nil:
				unit.ParentUnitID = &rootUnitID
			case errors.Is(err, sql.ErrNoRows):
				// Legacy top-level organizations did not have a root unit.
			default:
				return fmt.Errorf("find root organization unit: %w", err)
			}
		}

		input.Organization.IsRoot = false
		input.Organization.IsHierarchical = true
		if _, err := tx.NewInsert().Model(input.Organization).Exec(ctx); err != nil {
			return fmt.Errorf("create child organization: %w", err)
		}
		input.Namespace.NamespaceType = OrgNamespace
		if _, err := tx.NewInsert().Model(input.Namespace).Exec(ctx); err != nil {
			return fmt.Errorf("create child organization namespace: %w", err)
		}
		input.Organization.NamespaceID = input.Namespace.ID
		if _, err := tx.NewUpdate().Model(input.Organization).Column("namespace_id").WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("link child organization namespace: %w", err)
		}
		if err := insertOrganizationTags(ctx, tx, input.Organization.ID, input.TagIDs); err != nil {
			return err
		}
		unit.OrganizationID = input.Organization.ID
		if _, err := tx.NewInsert().Model(unit).Exec(ctx); err != nil {
			return fmt.Errorf("create organization unit: %w", err)
		}
		self := &OrganizationUnitClosure{
			RootOrganizationID: input.RootOrganizationID,
			AncestorUnitID:     unit.ID,
			DescendantUnitID:   unit.ID,
			Depth:              0,
		}
		if _, err := tx.NewInsert().Model(self).Exec(ctx); err != nil {
			return fmt.Errorf("create organization unit self closure: %w", err)
		}
		if unit.ParentUnitID != nil {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO organization_unit_closure
					(root_organization_id, ancestor_unit_id, descendant_unit_id, depth, created_at)
				SELECT root_organization_id, ancestor_unit_id, ?, depth + 1, CURRENT_TIMESTAMP
				FROM organization_unit_closure
				WHERE root_organization_id = ? AND descendant_unit_id = ?`,
				unit.ID, input.RootOrganizationID, *unit.ParentUnitID)
			if err != nil {
				return fmt.Errorf("create organization unit ancestor closure: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	created, err := selectUnitByUUID(ctx, s.db.Core, input.Organization.UUID.String())
	if err != nil {
		return nil, err
	}
	return pointerToOrganizationUnit(created), nil
}

// insertOrganizationTags inserts validated organization tags in the caller's transaction.
func insertOrganizationTags(ctx context.Context, tx bun.Tx, organizationID int64, tagIDs []int64) error {
	if len(tagIDs) == 0 {
		return nil
	}
	tags := make([]OrganizationTag, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		tags = append(tags, OrganizationTag{OrganizationID: organizationID, TagID: tagID})
	}
	if _, err := tx.NewInsert().Model(&tags).Exec(ctx); err != nil {
		return fmt.Errorf("create child organization tags: %w", err)
	}
	return nil
}

// pointerToOrganizationUnit maps a database record to a public response pointer.
func pointerToOrganizationUnit(unit *OrganizationUnit) *types.OrganizationUnit {
	result := toOrganizationUnit(unit)
	return &result
}

// Update atomically updates a root or child organization and its structural sort order.
func (s *organizationUnitStoreImpl) Update(ctx context.Context, input UpdateOrganizationUnitInput) (*types.OrganizationUnit, error) {
	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		unit, err := selectUnitForUpdate(ctx, tx, input.RootOrganizationID, input.UnitID)
		if err != nil {
			return err
		}
		if unit.OrganizationID != input.OrganizationID {
			return errors.New("organization unit references an unexpected organization")
		}

		organizationQuery := tx.NewUpdate().Model((*Organization)(nil)).
			Set("updated_at = CURRENT_TIMESTAMP").
			Where("organization.id = ? AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL", input.OrganizationID)
		if input.Nickname != nil {
			organizationQuery = organizationQuery.Set("name = ?", *input.Nickname)
		}
		if input.Description != nil {
			organizationQuery = organizationQuery.Set("description = ?", *input.Description)
		}
		if input.Homepage != nil {
			organizationQuery = organizationQuery.Set("homepage = ?", *input.Homepage)
		}
		if input.Logo != nil {
			organizationQuery = organizationQuery.Set("logo = ?", *input.Logo)
		}
		if input.Verified != nil {
			organizationQuery = organizationQuery.Set("verified = ?", *input.Verified)
		}
		if input.OrgType != nil {
			organizationQuery = organizationQuery.Set("org_type = ?", *input.OrgType)
		}
		if _, err := organizationQuery.Exec(ctx); err != nil {
			return fmt.Errorf("update organization: %w", err)
		}

		if input.TagIDs != nil {
			if _, err := tx.NewDelete().Model((*OrganizationTag)(nil)).
				Where("organization_id = ?", input.OrganizationID).Exec(ctx); err != nil {
				return fmt.Errorf("replace organization tags: %w", err)
			}
			if err := insertOrganizationTags(ctx, tx, input.OrganizationID, *input.TagIDs); err != nil {
				return err
			}
		}
		if input.SortOrder != nil {
			if _, err := tx.NewUpdate().Model((*OrganizationUnit)(nil)).
				Set("sort_order = ?, updated_at = CURRENT_TIMESTAMP", *input.SortOrder).
				Where("id = ? AND root_organization_id = ?", input.UnitID, input.RootOrganizationID).
				Exec(ctx); err != nil {
				return fmt.Errorf("update organization unit sort order: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	unit, err := selectUnitByUUID(ctx, s.db.Core, input.UnitUUID)
	if err != nil {
		return nil, err
	}
	if unit == nil {
		return nil, sql.ErrNoRows
	}
	return pointerToOrganizationUnit(unit), nil
}

// organizationHierarchyRelationshipRow stores one direct organization hierarchy edge returned by SQL.
type organizationHierarchyRelationshipRow struct {
	ParentOrganizationUUID string `bun:"parent_organization_uuid"`
	ChildOrganizationUUID  string `bun:"child_organization_uuid"`
}

// loadOrganizationHierarchyRelationships returns direct edges for selected child units.
// Deleted rows are included only when repairing an already-deleted hierarchy. A legacy first-level unit without a structural parent unit is attached to the root organization.
func loadOrganizationHierarchyRelationships(
	ctx context.Context,
	db bun.IDB,
	rootOrganizationID int64,
	childUnitIDs []int64,
	includeDeleted bool,
) ([]types.OrganizationHierarchyRelationship, error) {
	rows := make([]organizationHierarchyRelationshipRow, 0)
	query := db.NewSelect().
		TableExpr("organization_units AS child_unit").
		ColumnExpr("CAST(COALESCE(parent_organization.uuid, root_organization.uuid) AS TEXT) AS parent_organization_uuid").
		ColumnExpr("CAST(child_organization.uuid AS TEXT) AS child_organization_uuid").
		Join("JOIN organizations AS child_organization ON child_organization.id = child_unit.organization_id").
		Join("JOIN organizations AS root_organization ON root_organization.id = child_unit.root_organization_id").
		Join("LEFT JOIN organization_units AS parent_unit ON parent_unit.id = child_unit.parent_unit_id").
		Join("LEFT JOIN organizations AS parent_organization ON parent_organization.id = parent_unit.organization_id").
		Where("child_unit.root_organization_id = ?", rootOrganizationID).
		Where("child_unit.parent_unit_id IS NOT NULL OR child_unit.organization_id <> child_unit.root_organization_id").
		OrderExpr("child_unit.id ASC")
	if !includeDeleted {
		query = query.
			Where("child_unit.deleted_at IS NULL").
			Where("child_organization.deleted_at IS NULL").
			Where("root_organization.deleted_at IS NULL").
			Where("parent_unit.id IS NULL OR parent_unit.deleted_at IS NULL").
			Where("parent_organization.id IS NULL OR parent_organization.deleted_at IS NULL")
	}
	if len(childUnitIDs) > 0 {
		query = query.Where("child_unit.id IN (?)", bun.In(childUnitIDs))
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	result := make([]types.OrganizationHierarchyRelationship, 0, len(rows))
	for _, row := range rows {
		result = append(result, types.OrganizationHierarchyRelationship{
			ParentOrganizationUUID: row.ParentOrganizationUUID,
			ChildOrganizationUUID:  row.ChildOrganizationUUID,
		})
	}
	return result, nil
}

// Delete soft-deletes every child organization and unit in a subtree and removes derived closure rows.
func (s *organizationUnitStoreImpl) Delete(ctx context.Context, input DeleteOrganizationUnitInput) (*types.DeleteOrganizationUnitResp, error) {
	result := &types.DeleteOrganizationUnitResp{}
	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		unit, err := selectUnitForUpdate(ctx, tx, input.RootOrganizationID, input.UnitID)
		if err != nil {
			return err
		}
		if unit.OrganizationID == input.RootOrganizationID {
			return errorx.ReqParamInvalid(errors.New("top-level organization cannot be deleted through the organization unit API"), nil)
		}
		var closureIDs []int64
		if err := tx.NewSelect().Model((*OrganizationUnitClosure)(nil)).
			Column("descendant_unit_id").
			Where("root_organization_id = ? AND ancestor_unit_id = ?", input.RootOrganizationID, input.UnitID).
			Order("descendant_unit_id ASC").Scan(ctx, &closureIDs); err != nil {
			return fmt.Errorf("load closure subtree: %w", err)
		}
		var adjacencyIDs []int64
		if err := tx.NewRaw(`
			WITH RECURSIVE subtree AS (
				SELECT id FROM organization_units
				WHERE id = ? AND root_organization_id = ? AND deleted_at IS NULL
				UNION ALL
				SELECT child.id FROM organization_units AS child
				JOIN subtree AS parent ON child.parent_unit_id = parent.id
				WHERE child.root_organization_id = ? AND child.deleted_at IS NULL
			)
			SELECT id FROM subtree ORDER BY id`, input.UnitID, input.RootOrganizationID, input.RootOrganizationID).
			Scan(ctx, &adjacencyIDs); err != nil {
			return fmt.Errorf("load adjacency subtree: %w", err)
		}
		sort.Slice(closureIDs, func(i, j int) bool { return closureIDs[i] < closureIDs[j] })
		if !equalInt64Slices(closureIDs, adjacencyIDs) {
			return errors.New("organization unit closure is inconsistent with parent relationships")
		}
		if len(closureIDs) == 0 {
			return sql.ErrNoRows
		}
		result.DeletedHierarchyRelationships, err = loadOrganizationHierarchyRelationships(ctx, tx, input.RootOrganizationID, closureIDs, false)
		if err != nil {
			return fmt.Errorf("load subtree relationships for ReBAC cleanup: %w", err)
		}

		var organizationIDs []int64
		if err := tx.NewSelect().Model((*OrganizationUnit)(nil)).Column("organization_id").
			Where("root_organization_id = ? AND id IN (?)", input.RootOrganizationID, bun.In(closureIDs)).
			Scan(ctx, &organizationIDs); err != nil {
			return fmt.Errorf("load child organizations for subtree deletion: %w", err)
		}
		if err := tx.NewSelect().Model((*Organization)(nil)).ColumnExpr("CAST(organization.uuid AS TEXT)").
			Where("organization.id IN (?) AND organization.deleted_at IS NULL", bun.In(organizationIDs)).
			Order("id ASC").Scan(ctx, &result.DeletedOrganizationUUIDs); err != nil {
			return fmt.Errorf("load child organization UUIDs for SSO cleanup: %w", err)
		}
		result.DeletedOrganizationIDs = append(result.DeletedOrganizationIDs, organizationIDs...)
		result.DeletedReBACRelationships, err = loadOrganizationReBACCleanup(ctx, tx, organizationIDs)
		if err != nil {
			return fmt.Errorf("load subtree member and namespace relationships for ReBAC cleanup: %w", err)
		}
		var namespaces []organizationRepositoryNamespace
		if err := tx.NewSelect().Model((*Organization)(nil)).
			ColumnExpr("organization.namespace_id AS id").ColumnExpr("namespace.path AS path").
			ColumnExpr("CAST(organization.uuid AS TEXT) AS organization_uuid").
			Join("JOIN namespaces AS namespace ON namespace.id = organization.namespace_id").
			Where("organization.id IN (?) AND organization.deleted_at IS NULL", bun.In(organizationIDs)).
			OrderExpr("namespace.path ASC").Scan(ctx, &namespaces); err != nil {
			return fmt.Errorf("load child organization namespaces: %w", err)
		}
		namespaceIDs, namespacePaths := organizationNamespaceIDsAndPaths(namespaces)
		if err := lockOrganizationRepositoryNamespaces(ctx, tx, namespacePaths); err != nil {
			return err
		}
		repositoryIDs, err := findRepositoryIDsByNamespaces(ctx, tx, namespacePaths)
		if err != nil {
			return err
		}
		deletedRepositories, err := deleteRepositoriesByIDs(ctx, tx, repositoryIDs, s.repositoryDeletionJobClient)
		if err != nil {
			return err
		}
		result.DeletedRepositories = organizationDeletedRepositories(deletedRepositories, namespaces)
		if err := tx.NewSelect().Model((*Member)(nil)).ColumnExpr("COUNT(DISTINCT member.user_id)").
			Where("member.organization_id IN (?) AND member.deleted_at IS NULL", bun.In(organizationIDs)).
			Scan(ctx, &result.UsersAffected); err != nil {
			return fmt.Errorf("count affected organization members: %w", err)
		}
		memberResult, err := tx.NewUpdate().Model((*Member)(nil)).
			Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("member.organization_id IN (?) AND member.deleted_at IS NULL", bun.In(organizationIDs)).Exec(ctx)
		if err != nil {
			return fmt.Errorf("remove organization members: %w", err)
		}
		if affected, err := memberResult.RowsAffected(); err == nil {
			result.UnitMembersRemoved = int(affected)
		}
		if _, err := tx.NewDelete().Model((*OrganizationUnitClosure)(nil)).
			Where("root_organization_id = ?", input.RootOrganizationID).
			Where("ancestor_unit_id IN (?) OR descendant_unit_id IN (?)", bun.In(closureIDs), bun.In(closureIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("delete organization unit closure rows: %w", err)
		}
		unitResult, err := tx.NewUpdate().Model((*OrganizationUnit)(nil)).
			Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("ou.root_organization_id = ? AND ou.id IN (?) AND ou.deleted_at IS NULL", input.RootOrganizationID, bun.In(closureIDs)).Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete organization unit subtree: %w", err)
		}
		if affected, err := unitResult.RowsAffected(); err == nil {
			result.UnitsDeleted = int(affected)
		}
		if _, err := tx.NewUpdate().Model((*Organization)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("organization.id IN (?) AND organization.is_root = FALSE AND organization.deleted_at IS NULL", bun.In(organizationIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("soft-delete child organizations: %w", err)
		}
		result.OrganizationsDeleted = len(organizationIDs)
		if len(namespaceIDs) > 0 {
			if _, err := tx.NewUpdate().Model((*Namespace)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
				Where("namespace.id IN (?) AND namespace.deleted_at IS NULL", bun.In(namespaceIDs)).Exec(ctx); err != nil {
				return fmt.Errorf("soft-delete child organization namespaces: %w", err)
			}
		}
		if _, err := tx.NewDelete().Model((*OrganizationTag)(nil)).
			Where("organization_id IN (?)", bun.In(organizationIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("delete child organization tags: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

// organizationReBACCleanupRow is an internal projection used to collect deleted direct tuples.
type organizationReBACCleanupRow struct {
	OrganizationID   int64  `bun:"organization_id"`
	OrganizationUUID string `bun:"organization_uuid"`
	NamespaceUUID    string `bun:"namespace_uuid"`
	UserUUID         string `bun:"user_uuid"`
}

// loadOrganizationReBACCleanup loads organization namespaces and all retained member records.
// Deleted rows are intentionally included so a repeated deletion can finish an earlier partial ReBAC cleanup.
func loadOrganizationReBACCleanup(ctx context.Context, tx bun.Tx, organizationIDs []int64) ([]types.OrganizationReBACCleanup, error) {
	if len(organizationIDs) == 0 {
		return nil, nil
	}

	var organizations []organizationReBACCleanupRow
	if err := tx.NewRaw(`
		SELECT organization.id AS organization_id,
		       CAST(organization.uuid AS TEXT) AS organization_uuid,
		       COALESCE(namespace.uuid, '') AS namespace_uuid
		FROM organizations AS organization
		LEFT JOIN namespaces AS namespace ON namespace.id = organization.namespace_id
		WHERE organization.id IN (?)
		ORDER BY organization.id ASC`, bun.In(organizationIDs)).Scan(ctx, &organizations); err != nil {
		return nil, fmt.Errorf("load organization namespace relationships: %w", err)
	}

	cleanups := make([]types.OrganizationReBACCleanup, 0, len(organizations))
	indexes := make(map[int64]int, len(organizations))
	seenUsers := make(map[int64]map[string]struct{}, len(organizations))
	for _, organization := range organizations {
		indexes[organization.OrganizationID] = len(cleanups)
		seenUsers[organization.OrganizationID] = make(map[string]struct{})
		cleanups = append(cleanups, types.OrganizationReBACCleanup{
			OrganizationUUID: organization.OrganizationUUID,
			NamespaceUUID:    organization.NamespaceUUID,
		})
	}

	var members []organizationReBACCleanupRow
	if err := tx.NewRaw(`
		SELECT member.organization_id,
		       CAST(app_user.uuid AS TEXT) AS user_uuid
		FROM members AS member
		JOIN users AS app_user ON app_user.id = member.user_id
		WHERE member.organization_id IN (?)
		ORDER BY member.organization_id ASC, app_user.uuid ASC`, bun.In(organizationIDs)).Scan(ctx, &members); err != nil {
		return nil, fmt.Errorf("load organization member relationships: %w", err)
	}
	for _, member := range members {
		index, exists := indexes[member.OrganizationID]
		if !exists || member.UserUUID == "" {
			continue
		}
		if _, exists := seenUsers[member.OrganizationID][member.UserUUID]; exists {
			continue
		}
		seenUsers[member.OrganizationID][member.UserUUID] = struct{}{}
		cleanups[index].UserUUIDs = append(cleanups[index].UserUUIDs, member.UserUUID)
	}
	return cleanups, nil
}

// equalInt64Slices compares two sorted ID collections.
func equalInt64Slices(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// ListRoots returns child organizations directly attached to a top-level organization.
func (s *organizationUnitStoreImpl) ListRoots(ctx context.Context, input ListOrganizationUnitInput) ([]types.OrganizationUnitSummary, int, error) {
	var rootUnitID int64
	err := s.db.Core.NewSelect().Model((*OrganizationUnit)(nil)).Column("id").
		Where("ou.root_organization_id = ? AND ou.organization_id = ? AND ou.parent_unit_id IS NULL AND ou.deleted_at IS NULL", input.RootOrganizationID, input.RootOrganizationID).
		Scan(ctx, &rootUnitID)
	if err == nil {
		return s.listSummaries(ctx, input, "ou.parent_unit_id = ?", []any{rootUnitID})
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, fmt.Errorf("find root organization unit: %w", err)
	}
	return s.listSummaries(ctx, input, "ou.parent_unit_id IS NULL", nil)
}

// ListChildren returns only direct child organizations of one unit.
func (s *organizationUnitStoreImpl) ListChildren(ctx context.Context, input ListOrganizationUnitInput) ([]types.OrganizationUnitSummary, int, error) {
	return s.listSummaries(ctx, input, "ou.parent_unit_id = ?", []any{input.UnitID})
}

// listSummaries returns one hierarchy level with direct-child and distinct subtree-member counts.
func (s *organizationUnitStoreImpl) listSummaries(ctx context.Context, input ListOrganizationUnitInput, condition string, args []any) ([]types.OrganizationUnitSummary, int, error) {
	var units []OrganizationUnit
	query := s.db.Core.NewSelect().Model(&units).
		Join("JOIN organizations AS organization ON organization.id = ou.organization_id AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL").
		Where("ou.root_organization_id = ?", input.RootOrganizationID).
		Where(condition, args...)
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}
	err = unitSelectQuery(s.db.Core, &units).
		ColumnExpr(`(
			SELECT COUNT(*) FROM organization_units AS child
			WHERE child.root_organization_id = ou.root_organization_id
				AND child.parent_unit_id = ou.id AND child.deleted_at IS NULL
		) AS direct_children_count`).
		ColumnExpr(`(
			SELECT COUNT(DISTINCT member.user_id)
			FROM organization_unit_closure AS member_tree
			JOIN organization_units AS member_unit
				ON member_unit.id = member_tree.descendant_unit_id
				AND member_unit.root_organization_id = member_tree.root_organization_id
				AND member_unit.deleted_at IS NULL
			JOIN members AS member
				ON member.organization_id = member_unit.organization_id
				AND member.deleted_at IS NULL
			WHERE member_tree.root_organization_id = ou.root_organization_id
				AND member_tree.ancestor_unit_id = ou.id
		) AS subtree_member_count`).
		Where("ou.root_organization_id = ?", input.RootOrganizationID).
		Where(condition, args...).
		OrderExpr("ou.sort_order ASC, LOWER(organization.name) ASC, organization.uuid ASC").
		Limit(input.Per).Offset((input.Page - 1) * input.Per).Scan(ctx)
	return convertOrganizationUnitSummaries(units), total, errorx.HandleDBError(err, nil)
}

// convertOrganizationUnitSummaries maps one-level query records to public summary values.
func convertOrganizationUnitSummaries(units []OrganizationUnit) []types.OrganizationUnitSummary {
	result := make([]types.OrganizationUnitSummary, 0, len(units))
	for index := range units {
		result = append(result, toOrganizationUnitSummary(&units[index]))
	}
	return result
}
