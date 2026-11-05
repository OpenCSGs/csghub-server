package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AddOrganizationMembersInput contains the atomic membership write inputs.
type AddOrganizationMembersInput struct {
	RootOrganizationID int64
	OrganizationID     int64
	OrganizationUnitID *int64
	UserUUIDs          []string
	Role               types.UserRole
}

// RemoveOrganizationMembersInput contains direct relationships to remove.
type RemoveOrganizationMembersInput struct {
	RootOrganizationID int64
	OrganizationID     int64
	OrganizationUnitID *int64
	UserUUIDs          []string
}

// UpdateOrganizationMemberRoleInput contains one organization role update.
type UpdateOrganizationMemberRoleInput struct {
	RootOrganizationID int64
	OrganizationID     int64
	OrganizationUnitID *int64
	OrganizationUUID   string
	UserID             int64
	Role               types.UserRole
}

// ListOrganizationMembersInput contains organization member filters and pagination.
type ListOrganizationMembersInput struct {
	OrganizationID int64
	Search         string
	Role           types.UserRole
	Per            int
	Page           int
}

// SearchOrganizationMembersInput scopes a paginated user search to one root hierarchy.
type SearchOrganizationMembersInput struct {
	RootOrganizationID int64
	Search             string
	Per                int
	Page               int
}

// OrganizationUnitMemberStore provides hierarchy-aware operations over common organization memberships.
type OrganizationUnitMemberStore interface {
	UpsertMembers(ctx context.Context, input AddOrganizationMembersInput) (*types.OrganizationMemberMutationResp, error)
	RemoveMembers(ctx context.Context, input RemoveOrganizationMembersInput) (*types.OrganizationMemberMutationResp, error)
	UpdateMemberRole(ctx context.Context, input UpdateOrganizationMemberRoleInput) error
	ListMembers(ctx context.Context, input ListOrganizationMembersInput) ([]types.OrganizationMember, int, error)
	FindRole(ctx context.Context, organizationID, userID int64) (types.UserRole, error)
	// SearchMembers returns unique users and their direct memberships throughout one hierarchy.
	SearchMembers(ctx context.Context, input SearchOrganizationMembersInput) ([]types.OrganizationMemberSearchResult, int, error)
}

// organizationUnitMemberStoreImpl uses one database connection for hierarchy-aware membership transactions.
type organizationUnitMemberStoreImpl struct {
	db            *DB
	broadSearches organizationMemberSearchHints
	// rebacTupleJobClient records permission changes on this Store's business transaction.
	rebacTupleJobClient RebacTupleJobClient
}

// NewOrganizationUnitMemberStore creates a hierarchy-aware membership Store.
func NewOrganizationUnitMemberStore() OrganizationUnitMemberStore {
	return NewOrganizationUnitMemberStoreWithRebacTupleJobClient(defaultDB, nil)
}

// NewOrganizationUnitMemberStoreWithDB creates a membership Store with an explicit database.
func NewOrganizationUnitMemberStoreWithDB(db *DB) OrganizationUnitMemberStore {
	return NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, nil)
}

// NewOrganizationUnitMemberStoreWithRebacTupleJobClient injects the transactional permission job producer.
func NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db *DB, tupleJobs RebacTupleJobClient) OrganizationUnitMemberStore {
	return &organizationUnitMemberStoreImpl{db: db, rebacTupleJobClient: tupleJobs}
}

// UpsertMembers atomically creates or updates direct memberships while preserving an administrator.
func (s *organizationUnitMemberStoreImpl) UpsertMembers(ctx context.Context, input AddOrganizationMembersInput) (*types.OrganizationMemberMutationResp, error) {
	response := &types.OrganizationMemberMutationResp{
		MembersAdded: []string{},
		Skipped:      []string{},
	}
	if err := validateOrganizationUnitRole(input.Role); err != nil {
		return nil, err
	}
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockMemberSubjectsTx(ctx, tx, input.UserUUIDs); err != nil {
			return err
		}
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		if err := validateOrganizationMembershipTarget(ctx, tx, input.RootOrganizationID, input.OrganizationID, input.OrganizationUnitID); err != nil {
			return err
		}
		users, err := lockUsersByUUID(ctx, tx, input.UserUUIDs)
		if err != nil {
			return err
		}
		if input.Role != types.UserAdmin {
			adminUserIDs, err := lockOrganizationAdminUserIDs(ctx, tx, input.OrganizationID)
			if err != nil {
				return err
			}
			if err := ensureAdminRemains(adminUserIDs, users); err != nil {
				return err
			}
		}
		for _, user := range users {
			if err := upsertOrganizationMember(ctx, tx, input.OrganizationID, user.ID, input.Role); err != nil {
				return fmt.Errorf("upsert organization membership for user %s: %w", user.UUID, err)
			}
			response.MembersAdded = append(response.MembersAdded, user.UUID)
		}
		var events []types.RebacTuple
		for _, user := range users {
			event, err := memberTupleTx(ctx, tx, input.OrganizationID, user.ID, input.Role)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return enqueueRebacTuple(ctx, tx, s.rebacTupleJobClient, events...)
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// validateOrganizationUnitRole restricts hierarchy relationships to supported permissions.
func validateOrganizationUnitRole(role types.UserRole) error {
	switch role {
	case types.UserAdmin, types.UserWrite, types.UserRead:
		return nil
	default:
		return errorx.ReqParamInvalid(fmt.Errorf("unsupported organization unit role %q", role), nil)
	}
}

// validateOrganizationMembershipTarget verifies root memberships and child unit mappings.
func validateOrganizationMembershipTarget(ctx context.Context, tx bun.Tx, rootOrganizationID, organizationID int64, organizationUnitID *int64) error {
	if organizationUnitID == nil {
		if rootOrganizationID != organizationID {
			return errorx.ReqParamInvalid(errors.New("root organization membership cannot reference another organization"), nil)
		}
		var isRoot bool
		if err := tx.NewSelect().Model((*Organization)(nil)).Column("is_root").
			Where("organization.id = ? AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL", organizationID).Scan(ctx, &isRoot); err != nil {
			return err
		}
		if !isRoot {
			return errorx.ReqParamInvalid(errors.New("child organization membership requires an organization unit"), nil)
		}
		return nil
	}
	unit, err := selectUnitForUpdate(ctx, tx, rootOrganizationID, *organizationUnitID)
	if err != nil {
		return err
	}
	if unit.OrganizationID != organizationID {
		return errorx.ReqParamInvalid(errors.New("organization unit does not belong to organization"), nil)
	}
	return nil
}

// upsertOrganizationMember keeps an active relationship idempotent and updates changed roles.
func upsertOrganizationMember(ctx context.Context, tx bun.Tx, organizationID, userID int64, role types.UserRole) error {
	var relation Member
	err := tx.NewSelect().Model(&relation).
		Where("organization_id = ? AND user_id = ?", organizationID, userID).
		Scan(ctx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		relation = Member{
			OrganizationID: organizationID,
			UserID:         userID,
			Role:           string(role),
		}
		_, err = tx.NewInsert().Model(&relation).Exec(ctx)
		return err
	case err != nil:
		return err
	case relation.Role == string(role):
		return nil
	default:
		_, err = tx.NewUpdate().Model((*Member)(nil)).
			Set("role = ?, updated_at = CURRENT_TIMESTAMP", string(role)).
			Where("id = ?", relation.ID).Exec(ctx)
		return err
	}
}

// lockMemberSubjectsTx resolves subject IDs without row locks, then locks them in
// numeric order before organization locks. lockUsersByUUID rechecks active users
// after waiting, since account deletion may commit while these locks are acquired.
func lockMemberSubjectsTx(ctx context.Context, tx bun.Tx, userUUIDs []string) error {
	var ids []int64
	if err := tx.NewSelect().Model((*User)(nil)).Column("id").
		Where("uuid IN (?)", bun.In(uniqueStrings(userUUIDs))).Order("id ASC").Scan(ctx, &ids); err != nil {
		return err
	}
	for _, id := range ids {
		if err := lockUserRebacTx(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

// lockUsersByUUID resolves and locks every requested active user in stable order.
func lockUsersByUUID(ctx context.Context, tx bun.Tx, userUUIDs []string) ([]User, error) {
	uniqueUUIDs := uniqueStrings(userUUIDs)
	var users []User
	err := tx.NewSelect().Model(&users).
		Where("uuid IN (?)", bun.In(uniqueUUIDs)).
		Order("uuid ASC").
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock organization unit users: %w", err)
	}
	if len(users) != len(uniqueUUIDs) {
		found := make(map[string]struct{}, len(users))
		for _, user := range users {
			found[user.UUID] = struct{}{}
		}
		for _, userUUID := range uniqueUUIDs {
			if _, ok := found[userUUID]; !ok {
				return nil, errorx.ReqParamInvalid(fmt.Errorf("user %s does not exist", userUUID), nil)
			}
		}
	}
	return users, nil
}

// uniqueStrings removes duplicate batch values and returns stable ordering.
func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// lockOrganizationAdminUserIDs locks and returns every active administrator in one organization.
func lockOrganizationAdminUserIDs(ctx context.Context, tx bun.Tx, organizationID int64) ([]int64, error) {
	var userIDs []int64
	err := tx.NewSelect().Model((*Member)(nil)).
		Column("user_id").
		Where("organization_id = ? AND role = ? AND deleted_at IS NULL", organizationID, types.UserAdmin).
		Order("user_id ASC").
		For("UPDATE").
		Scan(ctx, &userIDs)
	if err != nil {
		return nil, fmt.Errorf("lock organization administrators: %w", err)
	}
	return userIDs, nil
}

// ensureAdminRemains rejects a batch that would revoke every active administrator role.
func ensureAdminRemains(adminUserIDs []int64, affectedUsers []User) error {
	if len(adminUserIDs) == 0 {
		return nil
	}
	affectedUserIDs := make(map[int64]struct{}, len(affectedUsers))
	for _, user := range affectedUsers {
		affectedUserIDs[user.ID] = struct{}{}
	}
	for _, adminUserID := range adminUserIDs {
		if _, affected := affectedUserIDs[adminUserID]; !affected {
			return nil
		}
	}
	return errorx.ErrLastOrgAdmin
}

// RemoveMembers removes only the requested users while preserving at least one organization administrator.
func (s *organizationUnitMemberStoreImpl) RemoveMembers(ctx context.Context, input RemoveOrganizationMembersInput) (*types.OrganizationMemberMutationResp, error) {
	response := &types.OrganizationMemberMutationResp{
		MembersRemoved: []string{},
		Skipped:        []string{},
	}
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockMemberSubjectsTx(ctx, tx, input.UserUUIDs); err != nil {
			return err
		}
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		if err := validateOrganizationMembershipTarget(ctx, tx, input.RootOrganizationID, input.OrganizationID, input.OrganizationUnitID); err != nil {
			return err
		}
		users, err := lockUsersByUUID(ctx, tx, input.UserUUIDs)
		if err != nil {
			return err
		}
		adminUserIDs, err := lockOrganizationAdminUserIDs(ctx, tx, input.OrganizationID)
		if err != nil {
			return err
		}
		if err := ensureAdminRemains(adminUserIDs, users); err != nil {
			return err
		}
		for _, user := range users {
			result, err := tx.NewDelete().Model((*Member)(nil)).
				Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", input.OrganizationID, user.ID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("remove organization membership for user %s: %w", user.UUID, err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				response.Skipped = append(response.Skipped, user.UUID)
			} else {
				response.MembersRemoved = append(response.MembersRemoved, user.UUID)
			}
		}
		var events []types.RebacTuple
		for _, user := range users {
			event, err := memberTupleTx(ctx, tx, input.OrganizationID, user.ID, "")
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return enqueueRebacTuple(ctx, tx, s.rebacTupleJobClient, events...)
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// UpdateMemberRole atomically preserves an administrator and updates one direct membership.
func (s *organizationUnitMemberStoreImpl) UpdateMemberRole(ctx context.Context, input UpdateOrganizationMemberRoleInput) error {
	if err := validateOrganizationUnitRole(input.Role); err != nil {
		return err
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUserRebacTx(ctx, tx, input.UserID); err != nil {
			return err
		}
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		if err := validateOrganizationMembershipTarget(ctx, tx, input.RootOrganizationID, input.OrganizationID, input.OrganizationUnitID); err != nil {
			return err
		}
		var currentRole types.UserRole
		err := tx.NewSelect().Model((*Member)(nil)).Column("role").
			Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", input.OrganizationID, input.UserID).
			For("UPDATE").
			Scan(ctx, &currentRole)
		if errors.Is(err, sql.ErrNoRows) {
			return errorx.OrganizationMemberNotFound(input.OrganizationUUID, input.UserID)
		}
		if err != nil {
			return fmt.Errorf("lock organization member role: %w", err)
		}
		if currentRole == types.UserAdmin && input.Role != types.UserAdmin {
			adminUserIDs, err := lockOrganizationAdminUserIDs(ctx, tx, input.OrganizationID)
			if err != nil {
				return err
			}
			if len(adminUserIDs) == 1 {
				return errorx.ErrLastOrgAdmin
			}
		}
		result, err := tx.NewUpdate().Model((*Member)(nil)).
			Set("role = ?, updated_at = CURRENT_TIMESTAMP", input.Role).
			Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", input.OrganizationID, input.UserID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("update organization member role: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("count updated organization member role: %w", err)
		}
		if affected == 0 {
			return errorx.OrganizationMemberNotFound(input.OrganizationUUID, input.UserID)
		}
		return enqueueMemberTx(ctx, tx, s.rebacTupleJobClient, input.OrganizationID, input.UserID, input.Role)
	})
}

// organizationMemberRow is the projection used by direct member queries.
type organizationMemberRow struct {
	UserID   int64          `bun:"user_id"`
	UserUUID string         `bun:"user_uuid"`
	Username string         `bun:"username"`
	Nickname string         `bun:"nickname"`
	Email    string         `bun:"email"`
	Avatar   string         `bun:"avatar"`
	UserRole types.UserRole `bun:"user_role"`
}

// ListMembers returns users assigned directly to one organization.
func (s *organizationUnitMemberStoreImpl) ListMembers(ctx context.Context, input ListOrganizationMembersInput) ([]types.OrganizationMember, int, error) {
	base := s.db.Core.NewSelect().
		Model((*Member)(nil)).
		Join("JOIN users AS u ON u.id = member.user_id AND u.deleted_at IS NULL").
		Join("JOIN organizations AS organization ON organization.id = member.organization_id AND organization.is_hierarchical = TRUE AND organization.deleted_at IS NULL").
		Where("member.organization_id = ? AND member.deleted_at IS NULL", input.OrganizationID)
	if input.Role != "" {
		base = base.Where("member.role = ?", input.Role)
	}
	if search := strings.TrimSpace(input.Search); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		base = base.Where("(LOWER(u.username) LIKE ? OR LOWER(u.name) LIKE ? OR LOWER(u.email) LIKE ?)", pattern, pattern, pattern)
	}
	total, err := base.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var rows []organizationMemberRow
	err = base.ColumnExpr("u.uuid AS user_uuid, u.username, u.name AS nickname, COALESCE(u.email, '') AS email, COALESCE(u.avatar, '') AS avatar, member.role AS user_role").
		OrderExpr("LOWER(u.username) ASC, u.uuid ASC").
		Limit(input.Per).Offset((input.Page-1)*input.Per).
		Scan(ctx, &rows)
	return convertOrganizationMembers(rows), total, err
}

// convertOrganizationMembers maps query rows to common response types.
func convertOrganizationMembers(rows []organizationMemberRow) []types.OrganizationMember {
	result := make([]types.OrganizationMember, 0, len(rows))
	for _, row := range rows {
		result = append(result, types.OrganizationMember{
			UserUUID: row.UserUUID, Username: row.Username, Nickname: row.Nickname, Email: row.Email, Avatar: row.Avatar,
			UserRole: row.UserRole,
		})
	}
	return result
}

// FindRole returns one user's active role in exactly one organization.
func (s *organizationUnitMemberStoreImpl) FindRole(ctx context.Context, organizationID, userID int64) (types.UserRole, error) {
	var role string
	err := s.db.Core.NewSelect().Model((*Member)(nil)).Column("role").
		Where("organization_id = ? AND user_id = ?", organizationID, userID).
		Scan(ctx, &role)
	return types.UserRole(role), errorx.HandleDBError(err, errorx.Ctx().Set("user_id", userID).Set("organization_id", organizationID))
}

// activeHierarchyMemberships selects direct memberships in active nodes of the requested tree.
func (s *organizationUnitMemberStoreImpl) activeHierarchyMemberships(rootID int64) *bun.SelectQuery {
	return s.db.Core.NewSelect().TableExpr("members AS m").
		Join("JOIN organizations AS o ON o.id = m.organization_id AND o.is_hierarchical = TRUE AND o.deleted_at IS NULL").
		Join("JOIN organization_units AS ou ON ou.organization_id = o.id AND ou.deleted_at IS NULL").
		Join("JOIN organizations AS root ON root.id = ou.root_organization_id AND root.is_root = TRUE AND root.is_hierarchical = TRUE AND root.deleted_at IS NULL").
		Where("ou.root_organization_id = ? AND m.deleted_at IS NULL", rootID)
}

// SearchMembers selects a bounded search strategy before loading complete direct memberships.
// Short terms and broad matches reuse a single filtered scan; hints never cache user data.
func (s *organizationUnitMemberStoreImpl) SearchMembers(ctx context.Context, input SearchOrganizationMembersInput) ([]types.OrganizationMemberSearchResult, int, error) {
	if input.Per <= 0 || input.Page <= 0 {
		return nil, 0, errorx.ReqParamInvalid(errors.New("per and page must be positive"), nil)
	}
	input.Search = strings.TrimSpace(input.Search)
	key := organizationMemberSearchHintKey{rootID: input.RootOrganizationID, search: input.Search}
	if !organizationMemberSearchCanUseTrigrams(input.Search) || s.broadSearches.contains(key, time.Now()) {
		return s.searchMembersFiltered(ctx, input)
	}
	routed, err := s.routeMemberSearch(ctx, input)
	if err != nil {
		return nil, 0, err
	}
	if routed.Mode == "broad_fallback" {
		// Routing returns placeholders for broad matches; the fresh filtered query supplies the response.
		results, total, err := s.searchMembersFiltered(ctx, input)
		if err == nil {
			s.broadSearches.remember(key, time.Now())
		}
		return results, total, err
	}
	if routed.Mode != "small_scope" && routed.Mode != "selective_index" {
		return nil, 0, fmt.Errorf("unknown organization member search mode %q", routed.Mode)
	}
	if len(routed.Rows) > 0 {
		if err := s.attachOrganizations(ctx, input.RootOrganizationID, routed.Rows); err != nil {
			return nil, 0, err
		}
	}
	return routed.Rows, routed.Total, nil
}

// searchMembersFiltered evaluates the original predicate once for both exact count and pagination.
func (s *organizationUnitMemberStoreImpl) searchMembersFiltered(ctx context.Context, input SearchOrganizationMembersInput) ([]types.OrganizationMemberSearchResult, int, error) {
	pattern := organizationMemberSearchPattern(input.Search)
	matches := s.db.Core.NewSelect().TableExpr("users AS u").
		ColumnExpr(organizationMemberSearchMatchColumns).
		Where("u.deleted_at IS NULL").
		Where("u.id IN (?)", s.activeHierarchyMemberships(input.RootOrganizationID).Column("m.user_id")).
		Where(organizationMemberSearchPredicate, pattern, pattern, pattern, pattern)
	result, err := s.scanMemberSearch(ctx, input,
		s.db.Core.NewRaw("WITH matched AS MATERIALIZED (?)", matches), s.db.Core.NewRaw("'filtered_scope'"))
	if err != nil {
		return nil, 0, err
	}
	if len(result.Rows) > 0 {
		if err := s.attachOrganizations(ctx, input.RootOrganizationID, result.Rows); err != nil {
			return nil, 0, err
		}
	}
	return result.Rows, result.Total, nil
}

// Search strategy limits bound probes independently of the number of matching users.
const (
	organizationMemberSearchScopeLimit     = 3000
	organizationMemberSearchCandidateLimit = 1000
	organizationMemberSearchHintLimit      = 1024
	organizationMemberSearchHintKeyBytes   = 256
	organizationMemberSearchHintTTL        = 30 * time.Second
	organizationMemberSearchPredicate      = "(LOWER(u.username) LIKE LOWER(?) ESCAPE '!' OR LOWER(u.name) LIKE LOWER(?) ESCAPE '!' OR LOWER(u.email) LIKE LOWER(?) ESCAPE '!' OR u.phone LIKE ? ESCAPE '!')"
	// Keep this expression identical to the shared migration so PostgreSQL can use its GIN index.
	organizationMemberSearchCombined = `LOWER(COALESCE(u.username, '')) || E'\n' || LOWER(COALESCE(u.name, '')) || E'\n' || LOWER(COALESCE(u.email, '')) || E'\n' || LOWER(COALESCE(u.phone, ''))`
	// Keep materialized matches narrow; profile fields are fetched only for the selected page.
	organizationMemberSearchMatchColumns = "u.id AS user_id, u.username, u.uuid AS user_uuid"
	organizationMemberSearchProjection   = "u.uuid AS user_uuid, u.username, COALESCE(u.name, '') AS nickname, COALESCE(u.email, '') AS email, COALESCE(u.avatar, '') AS avatar"
)

// organizationMemberSearchPattern preserves literal substring semantics in every search strategy.
func organizationMemberSearchPattern(search string) string {
	return "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(search) + "%"
}

// organizationMemberSearchCanUseTrigrams excludes terms without useful trigrams and common short phone prefixes.
func organizationMemberSearchCanUseTrigrams(search string) bool {
	run, longest, size := 0, 0, 0
	numeric := true
	for _, r := range search {
		size++
		if !unicode.IsDigit(r) {
			numeric = false
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	return longest >= 3 && !(numeric && size <= 3)
}

// organizationMemberSearchHintKey isolates routing hints by root and literal trimmed search term.
type organizationMemberSearchHintKey struct {
	rootID int64
	search string
}

// organizationMemberSearchHints is a bounded, concurrency-safe cache of broad-query hints only.
// Expiration changes optimization choices; every request still queries current database data.
type organizationMemberSearchHints struct {
	mu      sync.Mutex
	expires map[organizationMemberSearchHintKey]time.Time
}

// contains expires stale hints without extending their lifetime on reads.
func (h *organizationMemberSearchHints) contains(key organizationMemberSearchHintKey, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	until, ok := h.expires[key]
	if ok && !now.Before(until) {
		delete(h.expires, key)
		return false
	}
	return ok
}

// remember bounds both entry count and key size, evicting the oldest deadline when full.
func (h *organizationMemberSearchHints) remember(key organizationMemberSearchHintKey, now time.Time) {
	if len(key.search) > organizationMemberSearchHintKeyBytes {
		return
	}
	// TrimSpace can retain a large request's backing string; copy only the bounded cache key.
	key.search = strings.Clone(key.search)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.expires == nil {
		h.expires = make(map[organizationMemberSearchHintKey]time.Time)
	}
	if _, ok := h.expires[key]; !ok && len(h.expires) >= organizationMemberSearchHintLimit {
		var oldest organizationMemberSearchHintKey
		var deadline time.Time
		for existing, until := range h.expires {
			if !now.Before(until) {
				delete(h.expires, existing)
			} else if deadline.IsZero() || until.Before(deadline) {
				oldest, deadline = existing, until
			}
		}
		if len(h.expires) >= organizationMemberSearchHintLimit {
			delete(h.expires, oldest)
		}
	}
	h.expires[key] = now.Add(organizationMemberSearchHintTTL)
}

// organizationMemberSearchRouteResult holds one query's routing decision, exact count, and page.
type organizationMemberSearchRouteResult struct {
	Mode  string                                 `json:"mode"`
	Total int                                    `json:"total"`
	Rows  []types.OrganizationMemberSearchResult `json:"rows"`
}

// routeMemberSearch counts and pages bounded small-scope or selective matches in one SQL snapshot.
// OFFSET 0 keeps point lookups lateral. Broad candidates return a routing hint rather than materialized profiles.
func (s *organizationUnitMemberStoreImpl) routeMemberSearch(ctx context.Context, input SearchOrganizationMembersInput) (organizationMemberSearchRouteResult, error) {
	pattern := organizationMemberSearchPattern(input.Search)
	scope := s.activeHierarchyMemberships(input.RootOrganizationID).Column("m.user_id").Limit(organizationMemberSearchScopeLimit + 1)
	member := s.activeHierarchyMemberships(input.RootOrganizationID).ColumnExpr("1").Where("m.user_id = c.id").Limit(1)
	predicate := s.db.Core.NewRaw(organizationMemberSearchPredicate, pattern, pattern, pattern, pattern)
	ctes := s.db.Core.NewRaw(`
WITH scope_sample AS MATERIALIZED (?),
scope_size AS MATERIALIZED (SELECT count(*) n FROM scope_sample),
candidate_sample AS MATERIALIZED (
 SELECT u.id FROM users u
 WHERE (SELECT n FROM scope_size) > ? AND u.deleted_at IS NULL
  AND (?) LIKE LOWER(?) ESCAPE '!' LIMIT ?
),
candidate_size AS MATERIALIZED (SELECT count(*) n FROM candidate_sample),
matched AS MATERIALIZED (
 SELECT ? FROM (SELECT DISTINCT user_id FROM scope_sample) s
 JOIN LATERAL (SELECT id, username, name, email, phone, uuid, avatar FROM users u
  WHERE u.id = s.user_id AND u.deleted_at IS NULL OFFSET 0) u ON TRUE
 WHERE (SELECT n FROM scope_size) <= ? AND ?
 UNION ALL
 SELECT ? FROM candidate_sample c
 JOIN LATERAL (SELECT id, username, name, email, phone, uuid, avatar FROM users u
  WHERE u.id = c.id AND u.deleted_at IS NULL OFFSET 0) u ON TRUE
 JOIN LATERAL (?) active_member ON TRUE
 WHERE (SELECT n FROM scope_size) > ? AND (SELECT n FROM candidate_size) <= ? AND ?
)`, scope, organizationMemberSearchScopeLimit,
		bun.Safe(organizationMemberSearchCombined), pattern, organizationMemberSearchCandidateLimit+1,
		bun.Safe(organizationMemberSearchMatchColumns), organizationMemberSearchScopeLimit, predicate,
		bun.Safe(organizationMemberSearchMatchColumns), member, organizationMemberSearchScopeLimit,
		organizationMemberSearchCandidateLimit, predicate)
	mode := s.db.Core.NewRaw(`CASE WHEN (SELECT n FROM scope_size) <= ? THEN 'small_scope'
  WHEN (SELECT n FROM candidate_size) <= ? THEN 'selective_index' ELSE 'broad_fallback' END`,
		organizationMemberSearchScopeLimit, organizationMemberSearchCandidateLimit)
	return s.scanMemberSearch(ctx, input, ctes, mode)
}

// scanMemberSearch shares one snapshot and one filtered result between the exact count and page.
// Numeric operands prevent huge valid page numbers from overflowing bigint or Bun's int32 OFFSET.
// Both SQL fragments are built internally; caller input is formatted exclusively as query values.
func (s *organizationUnitMemberStoreImpl) scanMemberSearch(ctx context.Context, input SearchOrganizationMembersInput, ctes, mode *bun.RawQuery) (organizationMemberSearchRouteResult, error) {
	query := s.db.Core.NewRaw(`?,
total AS MATERIALIZED (SELECT count(*) total FROM matched)
SELECT json_build_object(
 'mode', ?, 'total', t.total,
 'rows', (SELECT COALESCE(json_agg(p ORDER BY LOWER(p.username), p.user_uuid), '[]'::json) FROM (
  SELECT ? FROM (
   SELECT user_id FROM matched ORDER BY LOWER(username), user_uuid LIMIT ?
   OFFSET (CASE WHEN t.total = 0 OR ?::numeric - 1 > (t.total - 1) / ? THEN t.total::numeric
    ELSE (?::numeric - 1) * ? END)
  ) page
  JOIN LATERAL (SELECT uuid, username, name, email, avatar FROM users u
   WHERE u.id = page.user_id AND u.deleted_at IS NULL OFFSET 0) u ON TRUE
 ) p)
) FROM total t`, ctes, mode, bun.Safe(organizationMemberSearchProjection), input.Per, input.Page, input.Per, input.Page, input.Per)
	var payload string
	if err := query.Scan(ctx, &payload); err != nil {
		return organizationMemberSearchRouteResult{}, err
	}
	var result organizationMemberSearchRouteResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return result, fmt.Errorf("decode organization member search page: %w", err)
	}
	if result.Rows == nil {
		result.Rows = []types.OrganizationMemberSearchResult{}
	}
	return result, nil
}

// organizationMemberSearchMembershipRow projects a membership for batch attachment to a user result.
type organizationMemberSearchMembershipRow struct {
	UserUUID string         `bun:"user_uuid"`
	UUID     string         `bun:"uuid"`
	Name     string         `bun:"name"`
	Nickname string         `bun:"nickname"`
	Role     types.UserRole `bun:"role"`
}

// attachOrganizations loads all direct memberships for a page without paginating the membership rows.
func (s *organizationUnitMemberStoreImpl) attachOrganizations(ctx context.Context, rootID int64, results []types.OrganizationMemberSearchResult) error {
	uuids := make([]string, 0, len(results))
	indexes := make(map[string]int, len(results))
	for i := range results {
		uuids = append(uuids, results[i].UserUUID)
		indexes[results[i].UserUUID] = i
		results[i].Organizations = []types.OrganizationMemberOrganization{}
	}
	var rows []organizationMemberSearchMembershipRow
	err := s.activeHierarchyMemberships(rootID).
		Join("JOIN users AS u ON u.id = m.user_id AND u.deleted_at IS NULL").
		ColumnExpr("u.uuid AS user_uuid, CAST(o.uuid AS TEXT) AS uuid, o.path AS name, COALESCE(o.name, '') AS nickname, m.role").
		Where("u.uuid IN (?)", bun.In(uuids)).
		OrderExpr("ou.sort_order ASC, LOWER(o.path) ASC, o.uuid ASC").Scan(ctx, &rows)
	if err != nil {
		return err
	}
	for _, row := range rows {
		i := indexes[row.UserUUID]
		results[i].Organizations = append(results[i].Organizations, types.OrganizationMemberOrganization{UUID: row.UUID, Name: row.Name, Nickname: row.Nickname, Role: row.Role})
	}
	return nil
}
