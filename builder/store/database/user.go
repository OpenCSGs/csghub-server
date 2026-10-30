package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// UserStore persists users and loads active roles in user queries.
// Pass UserWithAdminPermissions when the caller needs administrator membership or grants.
type UserStore interface {
	Index(ctx context.Context, options ...types.UserQueryOption) ([]User, error)
	IndexWithSearch(ctx context.Context, req types.UserListReq, options ...types.UserQueryOption) ([]User, int, error)
	// SearchByKeywordExcludingIDs searches users by name, username, or email while excluding the supplied user IDs.
	SearchByKeywordExcludingIDs(ctx context.Context, search string, excludedIDs []int64, limit int, options ...types.UserQueryOption) ([]User, error)
	FindByUsername(ctx context.Context, username string, options ...types.UserQueryOption) (User, error)
	FindByEmail(ctx context.Context, email string, options ...types.UserQueryOption) (User, error)
	// Update write the user data back to db. odlUserName should not be empty if username changed
	Update(ctx context.Context, user *User, oldUserName string) error
	ChangeUserName(ctx context.Context, username string, newUsername string) error
	Create(ctx context.Context, user *User, namespace *Namespace) error
	IsExist(ctx context.Context, username string) (bool, error)
	IsExistByUUID(ctx context.Context, uuid string) (bool, error)
	FindByGitAccessToken(ctx context.Context, token string, options ...types.UserQueryOption) (*User, error)
	FindByUUID(ctx context.Context, uuid string, options ...types.UserQueryOption) (*User, error)
	DeleteUserAndRelations(ctx context.Context, input User, req types.CloseAccountReq) error
	CountUsers(ctx context.Context) (int, error)
	UpdateVerifyStatus(ctx context.Context, uuid string, status types.VerifyStatus) error
	UpdateLabels(ctx context.Context, uuid string, labels []string) error
	FindByUUIDs(ctx context.Context, uuids []string, options ...types.UserQueryOption) ([]*User, error)
	SoftDeleteUserAndRelations(ctx context.Context, input User, req types.CloseAccountReq) (err error)
	IndexWithDeleted(ctx context.Context, options ...types.UserQueryOption) (users []User, err error)
	FindByUsernameWithDeleted(ctx context.Context, username string, options ...types.UserQueryOption) (User, error)
	IsExistWithDeleted(ctx context.Context, username string) (bool, error)
	GetEmails(ctx context.Context, per, page int) ([]string, int, error)
	GetUserUUIDs(ctx context.Context, per, page int) ([]string, int, error)
	GetAdminUserUUIDs(ctx context.Context) ([]string, error)
	GetAdminEmails(ctx context.Context) ([]string, error)
	UpdatePhone(ctx context.Context, userID int64, phone string, phoneArea string) error
	IndexWithCursor(ctx context.Context, req types.UserIndexReq, options ...types.UserQueryOption) (ch chan Wrapper, err error)
	FindByID(ctx context.Context, userID int64, options ...types.UserQueryOption) (User, error)
	// FindByIDs returns active users for a set of IDs in one query.
	FindByIDs(ctx context.Context, ids []int64, options ...types.UserQueryOption) ([]User, error)
}

// Implement the UserStore interface in UserStoreImpl
type UserStoreImpl struct {
	db                          *DB
	transactionCleanup          TransactionCleanup
	repositoryDeletionJobClient RepositoryDeletionJobClient
}

// TransactionCleanup removes edition-specific user relationships inside an existing transaction.
type TransactionCleanup func(ctx context.Context, tx bun.Tx, userID int64) error

func NewUserStore() UserStore {
	return &UserStoreImpl{
		db: defaultDB,
	}
}

func NewUserStoreWithDB(db *DB) UserStore {
	return &UserStoreImpl{
		db: db,
	}
}

// NewUserStoreWithDBAndDeletionJobClient creates a Store with transactional repository deletion jobs.
func NewUserStoreWithDBAndDeletionJobClient(db *DB, jobClient RepositoryDeletionJobClient) UserStore {
	return NewUserStoreWithDBCleanupAndDeletionJobClient(db, nil, jobClient)
}

// NewUserStoreWithCleanup creates a Store with an optional transactional relationship cleanup.
func NewUserStoreWithCleanup(cleanup TransactionCleanup) UserStore {
	return &UserStoreImpl{
		db:                 defaultDB,
		transactionCleanup: cleanup,
	}
}

// NewUserStoreWithDBAndCleanup creates a Store with explicit database and cleanup dependencies.
func NewUserStoreWithDBAndCleanup(db *DB, cleanup TransactionCleanup) UserStore {
	return NewUserStoreWithDBCleanupAndDeletionJobClient(db, cleanup, nil)
}

// NewUserStoreWithDBCleanupAndDeletionJobClient creates a Store with explicit transactional dependencies.
func NewUserStoreWithDBCleanupAndDeletionJobClient(db *DB, cleanup TransactionCleanup, jobClient RepositoryDeletionJobClient) UserStore {
	return &UserStoreImpl{
		db:                          db,
		transactionCleanup:          cleanup,
		repositoryDeletionJobClient: jobClient,
	}
}

// runTransactionCleanup locks the user before removing edition-specific relationships.
func (s *UserStoreImpl) runTransactionCleanup(ctx context.Context, tx bun.Tx, userID int64) error {
	if s.transactionCleanup == nil {
		return nil
	}
	var lockedUser User
	if err := tx.NewSelect().Model(&lockedUser).
		Column("id").
		WhereAllWithDeleted().
		Where("id = ?", userID).
		For("UPDATE").
		Scan(ctx); err != nil {
		return fmt.Errorf("failed to lock user %d for relationship cleanup: %w", userID, err)
	}
	if err := s.transactionCleanup(ctx, tx, userID); err != nil {
		return fmt.Errorf("failed to clean up user relationships for user %d: %w", userID, err)
	}
	return nil
}

// ensureUserIsNotLastOrganizationAdmin locks every active admin membership for the user's organizations
// and rejects the request when any organization would lose its final administrator.
func (s *UserStoreImpl) ensureUserIsNotLastOrganizationAdmin(ctx context.Context, tx bun.Tx, input User) error {
	var targetOrganizationIDs []int64
	if err := tx.NewSelect().
		Model((*Member)(nil)).
		Column("member.organization_id").
		Join("JOIN users AS target_user ON target_user.id = member.user_id AND target_user.deleted_at IS NULL").
		Join("JOIN organizations AS organization ON organization.id = member.organization_id AND organization.deleted_at IS NULL").
		Where("member.user_id = ?", input.ID).
		Where("member.deleted_at IS NULL").
		Where("member.role IN (?, ?)", string(types.UserAdmin), "owner").
		For("UPDATE").
		Scan(ctx, &targetOrganizationIDs); err != nil {
		return fmt.Errorf("lock user administrator memberships: %w", err)
	}
	if len(targetOrganizationIDs) == 0 {
		return nil
	}

	type organizationAdminRow struct {
		OrganizationID int64 `bun:"organization_id"`
	}
	var adminRows []organizationAdminRow
	if err := tx.NewSelect().
		Model((*Member)(nil)).
		Column("member.organization_id").
		Join("JOIN users AS admin_user ON admin_user.id = member.user_id AND admin_user.deleted_at IS NULL").
		Join("JOIN organizations AS organization ON organization.id = member.organization_id AND organization.deleted_at IS NULL").
		Where("member.organization_id IN (?)", bun.In(targetOrganizationIDs)).
		Where("member.deleted_at IS NULL").
		Where("member.role IN (?, ?)", string(types.UserAdmin), "owner").
		For("UPDATE").
		Scan(ctx, &adminRows); err != nil {
		return fmt.Errorf("lock organization administrators: %w", err)
	}

	counts := make(map[int64]int, len(targetOrganizationIDs))
	for _, row := range adminRows {
		counts[row.OrganizationID]++
	}
	seen := make(map[int64]struct{}, len(targetOrganizationIDs))
	for _, orgID := range targetOrganizationIDs {
		if _, ok := seen[orgID]; ok {
			continue
		}
		seen[orgID] = struct{}{}
		if counts[orgID] <= 1 {
			return errorx.LastOrgAdmin(
				errors.New("cannot delete the last administrator of an organization"),
				errorx.Ctx().Set("username", input.Username),
			)
		}
	}
	return nil
}

type User struct {
	ID       int64  `bun:",pk,autoincrement" json:"id"`
	GitID    int64  `bun:",notnull" json:"git_id"`
	NickName string `bun:"column:name,notnull" json:"name"`
	Username string `bun:",notnull,unique" json:"username"`
	Email    string `bun:",nullzero,unique" json:"email"`
	//git password
	Password     string        `bun:",notnull" json:"-"`
	AccessTokens []AccessToken `bun:"rel:has-many,join:id=user_id"`
	Namespaces   []Namespace   `bun:"rel:has-many,join:id=user_id" json:"namespace"`
	// RoleAssignments contains active role memberships loaded with the user.
	RoleAssignments []UserRole `bun:"rel:has-many,join:id=user_id" json:"-"`
	// TODO:add unique index after migration
	UUID string `bun:"," json:"uuid"`
	// user registered from default login page, from casdoor, etc. Possible values:
	//
	// - "default"
	// - "casdoor"
	RegProvider     string `bun:"," json:"reg_provider"`
	Gender          string `bun:"," json:"gender"`
	RoleMask        string `bun:"," json:"role_mask"`
	Phone           string `bun:"," json:"phone"`
	PhoneArea       string `bun:"," json:"phone_area"`
	PhoneVerified   bool   `bun:"," json:"phone_verified"`
	EmailVerified   bool   `bun:"," json:"email_verified"`
	LastLoginAt     string `bun:"," json:"last_login_at"`
	Avatar          string `bun:"," json:"avatar"`
	CompanyVerified bool   `bun:"," json:"company_verified"`
	//password for user registered without casdoor
	PasswordHash string `bun:"," json:"password_hash"`
	Homepage     string `bun:"," json:"homepage"`
	Bio          string `bun:"," json:"bio"`
	// allow user to change username once
	CanChangeUserName bool `bun:"can_change_user_name" json:"can_change_username"`

	// WechatID     string `bun:"," json:"wechat_id"`
	// GithubID     string `bun:"," json:"github_id"`
	// GitlabID     string `bun:"," json:"gitlab_id"`
	// SessionIP    string `bun:"," json:"session_ip"`
	// Nickname        string `bun:"," json:"nickname"`
	// GitToken        string `bun:"," json:"git_token"`
	// StarhubSynced   bool   `bun:"," json:"starhub_synced"`
	// GitTokenName string `bun:"," json:"git_token_name"`

	VerifyStatus types.VerifyStatus `bun:",notnull,default:'none'" json:"verify_status"` // none, pending, approved, rejected
	Labels       []string           `bun:",type:jsonb" json:"labels"`
	DeletedAt    time.Time          `bun:",soft_delete,nullzero"`
	RetainData   string             `bun:",nullzero" json:"retain_data"`
	Tags         []UserTag          `bun:"rel:has-many,join:id=user_id" json:"tags"`

	times
}

type Wrapper struct {
	Users []User `json:"users"`
	Err   error  `json:"error"`
}

// Roles returns active role codes, including roles without administrator definitions.
// Legacy RoleMask values are not a fallback for persisted assignments.
func (u *User) Roles() []string {
	roles := make([]string, 0, len(u.RoleAssignments))
	for _, assignment := range u.RoleAssignments {
		if assignment.DeletedAt.IsZero() {
			roles = append(roles, assignment.RoleCode)
		}
	}
	return roles
}

// HasAdminRole reports active membership in any defined administrator role.
func (u *User) HasAdminRole() bool {
	if !u.DeletedAt.IsZero() {
		return false
	}
	for _, assignment := range u.RoleAssignments {
		if assignment.DeletedAt.IsZero() && assignment.Role != nil && assignment.Role.IsAdmin {
			return true
		}
	}
	return false
}

// HasSuperAdminPermission requires an active administrator role with a stored global grant.
// A role's name alone never grants global administrator access.
func (u *User) HasSuperAdminPermission() bool {
	if !u.DeletedAt.IsZero() {
		return false
	}
	for _, assignment := range u.RoleAssignments {
		if !assignment.DeletedAt.IsZero() || assignment.Role == nil || !assignment.Role.IsAdmin {
			continue
		}
		for _, permission := range assignment.Role.Permissions {
			if permission.PermissionCode == types.AdminPermissionAll {
				return true
			}
		}
	}
	return false
}

// CanAdmin reports whether the loaded active roles grant super administrator permission.
// It requires the global wildcard grant and uses the same rule as SuperAdmin responses.
func (u *User) CanAdmin() bool {
	return u.HasSuperAdminPermission()
}

func (u *User) SetRoles(roles []string) {
	u.RoleMask = strings.Join(roles, ",")
}

// userQueryRelations selects role codes by default and administrator grants only on request.
func userQueryRelations(options ...types.UserQueryOption) func(*bun.SelectQuery) *bun.SelectQuery {
	return func(query *bun.SelectQuery) *bun.SelectQuery {
		if slices.Contains(options, types.UserWithAdminPermissions) {
			return query.Relation("RoleAssignments.Role.Permissions")
		}
		return query.Relation("RoleAssignments")
	}
}

// Index returns active users with roles and optionally administrator grants.
func (s *UserStoreImpl) Index(ctx context.Context, options ...types.UserQueryOption) (users []User, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&users).Apply(userQueryRelations(options...)).Scan(ctx)
	return users, errorx.HandleDBError(err, nil)
}

// IndexWithSearch filters and paginates active users, batch-loading role relationships for the page.
func (s *UserStoreImpl) IndexWithSearch(ctx context.Context, req types.UserListReq, options ...types.UserQueryOption) (users []User, count int, err error) {
	search := strings.ToLower(req.Search)
	query := s.db.Operator.Core.NewSelect().
		Model(&users).Apply(userQueryRelations(options...))
	if search != "" {
		if req.ExactMatch {
			query.Where("LOWER(username) = ? OR LOWER(email) = ? OR phone = ?", strings.ToLower(search), strings.ToLower(search), search)
		} else {
			query.Where("LOWER(username) like ? OR LOWER(email) like ? OR phone like ?", fmt.Sprintf("%%%s%%", strings.ToLower(search)), fmt.Sprintf("%%%s%%", strings.ToLower(search)), fmt.Sprintf("%%%s%%", search))
		}

	}
	if req.VerifyStatus != "" {
		query.Where("verify_status = ?", req.VerifyStatus)
	}
	if len(req.Labels) != 0 {
		labelsJSON, err := json.Marshal(req.Labels)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal labels: %w, %w", err, errorx.ErrInternalServerError)
		}
		query.Where("labels @> ?", string(labelsJSON))
	}
	count, err = query.Count(ctx)
	if err != nil {
		return users, count, errorx.HandleDBError(err, nil)
	}
	if req.SortBy == "" {
		req.SortBy = "id"
	}
	if req.SortOrder == "" {
		req.SortOrder = "asc"
	}
	query.Order(fmt.Sprintf("%s %s", req.SortBy, req.SortOrder)).Limit(req.Per).Offset((req.Page - 1) * req.Per)
	err = query.Scan(ctx)
	return users, count, errorx.HandleDBError(err, nil)
}

// SearchByKeywordExcludingIDs searches users by display name, username, or email.
func (s *UserStoreImpl) SearchByKeywordExcludingIDs(ctx context.Context, search string, excludedIDs []int64, limit int, options ...types.UserQueryOption) (users []User, err error) {
	if limit <= 0 {
		limit = 20
	}
	pattern := fmt.Sprintf("%%%s%%", strings.ToLower(search))
	query := s.db.Operator.Core.NewSelect().Model(&users).Apply(userQueryRelations(options...)).Where(
		"LOWER(name) LIKE ? OR LOWER(username) LIKE ? OR LOWER(email) LIKE ?", pattern, pattern, pattern,
	)
	if len(excludedIDs) > 0 {
		query = query.Where("id NOT IN (?)", bun.In(excludedIDs))
	}
	err = query.Order("id ASC").Limit(limit).Scan(ctx)
	return users, errorx.HandleDBError(err, nil)
}

// FindByUsername returns an active user with namespaces and role relationships.
func (s *UserStoreImpl) FindByUsername(ctx context.Context, username string, options ...types.UserQueryOption) (user User, err error) {
	user.Username = username
	err = s.db.Operator.Core.NewSelect().
		Model(&user).Relation("Namespaces").Apply(userQueryRelations(options...)).
		Where("username = ?", username).
		Scan(ctx)
	return user, errorx.HandleDBError(err, map[string]interface{}{"username": username})
}

// FindByEmail returns an active user with role relationships by email.
func (s *UserStoreImpl) FindByEmail(ctx context.Context, email string, options ...types.UserQueryOption) (user User, err error) {
	user.Email = email
	err = s.db.Operator.Core.NewSelect().Model(&user).Apply(userQueryRelations(options...)).Where("email = ?", email).Scan(ctx)
	return user, errorx.HandleDBError(err, nil)
}

// FindByID returns an active user with role relationships by ID.
func (s *UserStoreImpl) FindByID(ctx context.Context, id int64, options ...types.UserQueryOption) (user User, err error) {
	user.ID = id
	err = s.db.Operator.Core.NewSelect().Model(&user).Apply(userQueryRelations(options...)).WherePK().Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

// FindByIDs returns active users and batch-loads role relationships for a set of IDs.
func (s *UserStoreImpl) FindByIDs(ctx context.Context, ids []int64, options ...types.UserQueryOption) (users []User, err error) {
	users = make([]User, 0)
	if len(ids) == 0 {
		return users, nil
	}
	err = s.db.Operator.Core.NewSelect().Model(&users).Apply(userQueryRelations(options...)).
		Where("id IN (?)", bun.In(ids)).
		Order("id ASC").Scan(ctx)
	return users, errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) Update(ctx context.Context, user *User, oldUserName string) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if len(oldUserName) > 0 && oldUserName != user.Username {
			if err = assertAffectedOneRow(tx.NewUpdate().Model((*Namespace)(nil)).
				Set("path = ?", user.Username).
				Set("updated_at = now()").
				Where("path = ?", oldUserName).
				Exec(ctx)); err != nil {
				return fmt.Errorf("failed to change namespace from '%s' to '%s' in db, error:%w", oldUserName, user.Username, err)
			}
		}

		err = assertAffectedOneRow(tx.NewUpdate().
			Model(user).
			WherePK().
			Exec(ctx),
		)
		if err != nil {
			return fmt.Errorf("failed to change username from '%s' to '%s' in db, error:%w", oldUserName, user.Username, err)
		}
		return nil
	})
	return errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) ChangeUserName(ctx context.Context, username string, newUsername string) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(tx.NewUpdate().Model((*Namespace)(nil)).
			Set("path = ?", newUsername).
			Set("updated_at = now()").
			Where("path = ?", username).
			Exec(ctx)); err != nil {
			return fmt.Errorf("failed to change namespace from '%s' to '%s' in db, error:%w", username, newUsername, err)
		}

		err = assertAffectedOneRow(tx.NewUpdate().
			Model((*User)(nil)).
			Where("username = ?", username).
			Set("username = ?", newUsername).
			Set("can_change_user_name = false").
			Set("updated_at = now()").
			Exec(ctx))
		if err != nil {
			return fmt.Errorf("failed to change username from '%s' to '%s' in db, error:%w", username, newUsername, err)
		}
		return nil
	})
	return errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) Create(ctx context.Context, user *User, namespace *Namespace) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err = assertAffectedOneRow(tx.NewInsert().Model(user).Exec(ctx)); err != nil {
			return err
		}
		namespace.UserID = user.ID
		namespace.NamespaceType = UserNamespace
		if err = assertAffectedOneRow(tx.NewInsert().Model(namespace).Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
	return errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) IsExist(ctx context.Context, username string) (exists bool, err error) {
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model((*User)(nil)).
		Where("username =?", username).
		Exists(ctx)
	return exists, errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) IsExistWithDeleted(ctx context.Context, username string) (exists bool, err error) {
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model((*User)(nil)).
		WhereAllWithDeleted().
		Where("username =?", username).
		Exists(ctx)
	return exists, errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) IsExistByUUID(ctx context.Context, uuid string) (exists bool, err error) {
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model((*User)(nil)).
		Where("uuid =?", uuid).
		Exists(ctx)
	return exists, errorx.HandleDBError(err, nil)
}

// FindByGitAccessToken retrieves an active user and role relationships through an unexpired Git token.
func (s *UserStoreImpl) FindByGitAccessToken(ctx context.Context, token string, options ...types.UserQueryOption) (*User, error) {
	var user User
	err := s.db.Operator.Core.NewSelect().
		Model(&user).Apply(userQueryRelations(options...)).
		Join(`JOIN access_tokens AS t ON t.user_id = "user".id`).
		Where("t.token = ? and t.is_active = true and (t.expired_at is null or t.expired_at > now()) and t.app = 'git'", token).
		Scan(ctx)

	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &user, nil
}

// FindByUUID returns an active user with namespaces and role relationships by UUID.
func (s *UserStoreImpl) FindByUUID(ctx context.Context, uuid string, options ...types.UserQueryOption) (*User, error) {
	var user User
	err := s.db.Operator.Core.NewSelect().
		Model(&user).Relation("Namespaces").Apply(userQueryRelations(options...)).
		Where("uuid = ?", uuid).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &user, nil
}

func (s *UserStoreImpl) GetActiveUserCount(ctx context.Context) (int, error) {
	count, err := s.db.Operator.Core.
		NewSelect().
		Model(&User{}).
		Count(ctx)
	return count, errorx.HandleDBError(err, nil)
}

// DeleteUserAndRelations protects active administrators before permanently deleting a user and related data.
func (s *UserStoreImpl) DeleteUserAndRelations(ctx context.Context, input User, req types.CloseAccountReq) (err error) {
	exists, err := s.IsExistWithDeleted(ctx, input.Username)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %v", err)
	}
	if !exists {
		return errorx.ErrDatabaseNoRows
	}

	err = s.db.Operator.Core.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(ctx context.Context, tx bun.Tx) error {
		if err := protectAdministratorFromUserDeletion(ctx, tx, input.ID); err != nil {
			return err
		}
		if err := s.runTransactionCleanup(ctx, tx, input.ID); err != nil {
			return err
		}
		if err := s.ensureUserIsNotLastOrganizationAdmin(ctx, tx, input); err != nil {
			return err
		}
		if !req.Repository {
			if err := lockAndSoftDeleteUserRepositoryNamespace(ctx, tx, input.ID); err != nil {
				return err
			}
			var repoIDs []int64
			if err := tx.NewSelect().Column("id").Model(&Repository{}).Where("user_id = ?", input.ID).Scan(ctx, &repoIDs); err != nil {
				return fmt.Errorf("failed to get user repo ids: %v", err)
			}
			if _, err := deleteRepositoriesByIDs(ctx, tx, repoIDs, s.repositoryDeletionJobClient); err != nil {
				return fmt.Errorf("failed to delete user repositories for user ID %d: %w", input.ID, err)
			}
		}

		// Keep the user row until repository deletion jobs have snapshotted the
		// stable user UUID used by Repository ReBAC.
		if err = assertAffectedOneRow(tx.NewDelete().Model(&input).Where("id = ?", input.ID).ForceDelete().Exec(ctx)); err != nil {
			return fmt.Errorf("failed to delete user %d: %v", input.ID, err)
		}

		if !req.Discussion {
			// Delete user's discussions
			if _, err := tx.NewDelete().Model(&Discussion{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user discussions for user ID %d: %v", input.ID, err)
			}

			// Delete user's comments
			if _, err := tx.NewDelete().Model(&Comment{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user discussions for user ID %d: %v", input.ID, err)
			}
		}
		// Delete user's access token
		if _, err := tx.NewDelete().Model(&AccessToken{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user access tokens for user ID %d: %v", input.ID, err)
		}
		// Delete user's member
		if _, err := tx.NewDelete().Model(&Member{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user members for user ID %d: %v", input.ID, err)
		}
		// Delete user's account_sync_quota
		if _, err := tx.NewDelete().Model(&AccountSyncQuota{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user account sync quotas for user ID %d: %v", input.ID, err)
		}
		// Delete user's ssh keys
		if _, err := tx.NewDelete().Model(&SSHKey{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user ssh keys for user ID %d: %v", input.ID, err)
		}
		// Delete user's user likes
		if _, err := tx.NewDelete().Model(&UserLike{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user likes for user ID %d: %v", input.ID, err)
		}
		// Delete user's prompt conversations
		if _, err := tx.NewDelete().Model(&PromptConversation{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user prompt conversations for user ID %d:  %v", input.ID, err)
		}

		return nil
	})
	return errorx.HandleDBError(err, nil)
}

// lockAndSoftDeleteUserRepositoryNamespace serializes repository creation and
// ownership transfer with permanent user deletion. The tombstone is retained
// so a writer that was waiting on the namespace lock cannot create an orphan
// after the user row has been removed.
func lockAndSoftDeleteUserRepositoryNamespace(ctx context.Context, tx bun.Tx, userID int64) error {
	var namespace Namespace
	query := tx.NewSelect().Model(&namespace).WhereAllWithDeleted().
		Where("user_id = ?", userID).
		OrderExpr("deleted_at IS NULL DESC").
		Order("id ASC").
		Limit(1)
	if tx.Dialect().Name() == dialect.PG {
		query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("lock user repository namespace for user ID %d: %w", userID, err)
	}
	if !namespace.DeletedAt.IsZero() {
		return nil
	}
	if err := assertAffectedOneRow(tx.NewDelete().Model(&namespace).WherePK().Exec(ctx)); err != nil {
		return fmt.Errorf("soft-delete user repository namespace for user ID %d: %w", userID, err)
	}
	return nil
}

// CountUsers counts users whose personal namespace is not mirrored, i.e.
// users created by normal registration rather than imported via multi-sync.
// It joins the user's personal namespace (namespace_type = user) and excludes
// mirrored ones (see component/multi_sync.go createUser), so the license
// "users in subscription" metric reflects real local accounts only.
func (s *UserStoreImpl) CountUsers(ctx context.Context) (int, error) {
	var users []User
	q := s.db.Operator.Core.NewSelect().Model(&users).
		Join(fmt.Sprintf(`JOIN namespaces AS ns ON ns.user_id = "user".id AND ns.namespace_type = '%s' AND ns.deleted_at IS NULL`, UserNamespace)).
		Where("ns.mirrored = false")
	count, err := q.Count(ctx)
	if err != nil {
		return 0, errorx.HandleDBError(err, nil)
	}
	return count, nil
}

func (s *UserStoreImpl) UpdateVerifyStatus(ctx context.Context, uuid string, status types.VerifyStatus) error {
	_, err := s.db.Operator.Core.
		NewUpdate().
		Model(&User{}).
		Set("verify_status = ?", status).
		Where("uuid = ?", uuid).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) UpdateLabels(ctx context.Context, uuid string, labels []string) error {
	err := assertAffectedOneRow(s.db.Operator.Core.NewUpdate().
		Model((*User)(nil)).
		Where("uuid = ?", uuid).
		Set("labels = ?", labels).
		Exec(ctx))

	return errorx.HandleDBError(err, nil)
}

// FindByUUIDs batch-loads active users, namespaces, and role relationships for UUIDs.
func (s *UserStoreImpl) FindByUUIDs(ctx context.Context, uuids []string, options ...types.UserQueryOption) ([]*User, error) {
	var users []*User
	if len(uuids) == 0 {
		return users, nil
	}

	err := s.db.Operator.Core.NewSelect().
		Model(&users).Relation("Namespaces").Apply(userQueryRelations(options...)).
		Where("uuid IN (?)", bun.In(uuids)).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return users, nil
}

// IndexWithDeleted includes deleted users while loading only active role assignments.
func (s *UserStoreImpl) IndexWithDeleted(ctx context.Context, options ...types.UserQueryOption) (users []User, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&users).Apply(userQueryRelations(options...)).WhereAllWithDeleted().Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

// SoftDeleteUserAndRelations applies the same administrator protection before closing a user account.
func (s *UserStoreImpl) SoftDeleteUserAndRelations(ctx context.Context, input User, req types.CloseAccountReq) (err error) {
	exists, err := s.IsExist(ctx, input.Username)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %w", err)
	}
	if !exists {
		return errorx.ErrDatabaseNoRows
	}
	err = s.db.Operator.Core.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(ctx context.Context, tx bun.Tx) error {
		if err := protectAdministratorFromUserDeletion(ctx, tx, input.ID); err != nil {
			return err
		}
		if err := s.runTransactionCleanup(ctx, tx, input.ID); err != nil {
			return err
		}
		if err := s.ensureUserIsNotLastOrganizationAdmin(ctx, tx, input); err != nil {
			return err
		}
		// Update ueser retain data
		mReq, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal close account request: %w", err)
		}
		if err = assertAffectedOneRow(tx.NewUpdate().Model(&input).Where("id = ?", input.ID).Set("retain_data = ?", string(mReq)).Exec(ctx)); err != nil {
			return fmt.Errorf("failed to update user retain_data: %w", err)
		}

		// Delete user
		if err = assertAffectedOneRow(tx.NewDelete().Model(&input).Where("id = ?", input.ID).Exec(ctx)); err != nil {
			return fmt.Errorf("failed to delete user %d: %v", input.ID, err)
		}

		if req.Repository {
			// Get user's repository_ids
			var repoIDs []int64
			if err := s.db.Operator.Core.NewSelect().Column("id").Model(&Repository{}).Where("user_id = ?", input.ID).Scan(ctx, &repoIDs); err != nil {
				return fmt.Errorf("failed to get user repo ids: %v", err)
			}
			// Delete user's repo
			if _, err = tx.NewDelete().Model(&Repository{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user repos for user ID %d: %v", input.ID, err)
			}
		}

		if req.Discussion {
			// Delete user's discussions
			if _, err := tx.NewDelete().Model(&Discussion{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user discussions for user ID %d: %v", input.ID, err)
			}

			// Delete user's comments
			if _, err := tx.NewDelete().Model(&Comment{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user discussions for user ID %d: %v", input.ID, err)
			}
		}

		// Delete user's lfs locks
		if _, err := tx.NewDelete().Model(&LfsLock{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user lfs locks for user ID %d:  %v", input.ID, err)
		}

		// Delete user's namespace
		if _, err := tx.NewDelete().Model(&Namespace{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user namespaces for user ID %d:  %v", input.ID, err)
		}

		// Delete user's access token
		if _, err := tx.NewDelete().Model(&AccessToken{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user access tokens for user ID %d: %v", input.ID, err)
		}

		// Delete user's member
		if _, err := tx.NewDelete().Model(&Member{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user members for user ID %d: %v", input.ID, err)
		}

		// Delete user's ssh keys
		if _, err := tx.NewDelete().Model(&SSHKey{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user ssh keys for user ID %d: %v", input.ID, err)
		}
		// Delete user's user likes
		if _, err := tx.NewDelete().Model(&UserLike{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user likes for user ID %d: %v", input.ID, err)
		}
		return nil
	})

	return errorx.HandleDBError(err, nil)
}

// FindByUsernameWithDeleted includes deleted users while loading active role assignments.
func (s *UserStoreImpl) FindByUsernameWithDeleted(ctx context.Context, username string, options ...types.UserQueryOption) (user User, err error) {
	user.Username = username
	err = s.db.Operator.Core.NewSelect().Model(&user).Apply(userQueryRelations(options...)).WhereAllWithDeleted().Where("username = ?", username).Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

// GetEmails retrieves all user emails from the database.
func (s *UserStoreImpl) GetEmails(ctx context.Context, per, page int) (emails []string, count int, err error) {
	query := s.db.Operator.Core.NewSelect().
		Model((*User)(nil)).
		Column("email").
		Where("email IS NOT NULL AND email != ''")
	count, err = query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count emails: %w", err)
	}
	query = query.Order("id ASC").Limit(per).Offset((page - 1) * per)
	err = query.Scan(ctx, &emails)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, 0, fmt.Errorf("failed to get user emails: %w", err)
	}
	return
}

func (s *UserStoreImpl) GetUserUUIDs(ctx context.Context, per, page int) (uuids []string, count int, err error) {
	query := s.db.Operator.Core.NewSelect().
		Model((*User)(nil)).
		Column("uuid").
		Where("uuid IS NOT NULL AND uuid != ''")
	count, err = query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}
	query = query.Order("id ASC").Limit(per).Offset((page - 1) * per)
	err = query.Scan(ctx, &uuids)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}
	return uuids, count, nil
}

// GetAdminUserUUIDs returns active super administrators with a nonempty UUID.
func (s *UserStoreImpl) GetAdminUserUUIDs(ctx context.Context) ([]string, error) {
	adminUUIDs := make([]string, 0)
	err := s.db.Operator.Core.NewSelect().
		Model((*User)(nil)).
		Column("uuid").
		Where("uuid IS NOT NULL AND uuid != ''").
		Where("EXISTS (?)", s.db.Operator.Core.NewSelect().Model((*UserRole)(nil)).
			ColumnExpr("1").
			Join("JOIN roles AS ar ON ar.role_code = ur.role_code").
			Where("ar.is_admin = ?", true).
			Join("JOIN admin_role_permissions AS arp ON arp.role_code = ar.role_code").
			Where("ur.user_id = ?", bun.Ident("user.id")).
			Where("arp.permission_code = ?", types.AdminPermissionAll)).
		Scan(ctx, &adminUUIDs)
	return adminUUIDs, errorx.HandleDBError(err, nil)
}

// GetAdminEmails returns active super administrators with a nonempty email.
func (s *UserStoreImpl) GetAdminEmails(ctx context.Context) ([]string, error) {
	adminEmails := make([]string, 0)
	err := s.db.Operator.Core.NewSelect().
		Model((*User)(nil)).
		Column("email").
		Where("email IS NOT NULL AND email != ''").
		Where("EXISTS (?)", s.db.Operator.Core.NewSelect().Model((*UserRole)(nil)).
			ColumnExpr("1").
			Join("JOIN roles AS ar ON ar.role_code = ur.role_code").
			Where("ar.is_admin = ?", true).
			Join("JOIN admin_role_permissions AS arp ON arp.role_code = ar.role_code").
			Where("ur.user_id = ?", bun.Ident("user.id")).
			Where("arp.permission_code = ?", types.AdminPermissionAll)).
		Scan(ctx, &adminEmails)
	return adminEmails, errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) UpdatePhone(ctx context.Context, userID int64, phone string, phoneArea string) error {
	_, err := s.db.Operator.Core.
		NewUpdate().
		Model(&User{}).
		Set("phone = ?", phone).
		Set("phone_area = ?", phoneArea).
		Where("id = ?", userID).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// IndexWithCursor exports active users in batches with their role relationships.
func (s *UserStoreImpl) IndexWithCursor(ctx context.Context, req types.UserIndexReq, options ...types.UserQueryOption) (chan Wrapper, error) {
	batchSize := req.Per
	if batchSize == 0 {
		batchSize = 100
	}

	result := make(chan Wrapper)
	go func() {
		defer close(result)
		err := s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
			query := tx.Core.NewSelect().Model(&User{})
			if req.Search != "" {
				query.Where("LOWER(username) like ? OR LOWER(email) like ? OR phone like ?",
					fmt.Sprintf("%%%s%%", strings.ToLower(req.Search)),
					fmt.Sprintf("%%%s%%", strings.ToLower(req.Search)),
					fmt.Sprintf("%%%s%%", req.Search),
				)
			}

			if req.VerifyStatus != "" {
				query.Where("verify_status = ?", req.VerifyStatus)
			}

			if len(req.Labels) != 0 {
				labelsJSON, err := json.Marshal(req.Labels)
				if err != nil {
					return fmt.Errorf("failed to marshal labels: %w, %w", err, errorx.ErrInternalServerError)
				}
				query.Where("labels @> ?", string(labelsJSON))
			}
			query.Order("id ASC")

			sqlStr := query.String()
			if _, err := tx.Core.ExecContext(ctx, fmt.Sprintf("DECLARE user_cursor CURSOR WITH HOLD FOR %s", sqlStr)); err != nil {
				slog.Error("failed to declare user cursor", "error", err)
				return errorx.ErrInternalServerError
			}

			for {
				select {
				case <-ctx.Done():
					return nil
				default:
				}
				rows, err := tx.Core.QueryContext(ctx, fmt.Sprintf("FETCH FORWARD %d FROM user_cursor", batchSize))
				if err != nil {
					return err
				}

				var batch []User
				if err := s.db.BunDB.ScanRows(ctx, rows, &batch); err != nil {
					return err
				}

				if len(batch) == 0 {
					break
				}
				if err := loadUserRoleAssignments(ctx, tx.Core, batch, options...); err != nil {
					return err
				}

				result <- Wrapper{Users: batch}
			}

			if _, err := tx.Core.ExecContext(ctx, `CLOSE user_cursor`); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			slog.Error("failed to index users with cursor", "error", err)
			result <- Wrapper{Err: err}
		}
	}()

	return result, nil
}

// loadUserRoleAssignments batch-loads active roles and optional grants for raw cursor results.
func loadUserRoleAssignments(ctx context.Context, db bun.IDB, users []User, options ...types.UserQueryOption) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(users))
	indexes := make(map[int64]int, len(users))
	for i := range users {
		ids = append(ids, users[i].ID)
		indexes[users[i].ID] = i
		users[i].RoleAssignments = nil
	}
	var assignments []UserRole
	query := db.NewSelect().Model(&assignments)
	if slices.Contains(options, types.UserWithAdminPermissions) {
		query.Relation("Role.Permissions")
	}
	if err := query.Where("ur.user_id IN (?)", bun.In(ids)).Order("ur.id ASC").Scan(ctx); err != nil {
		return fmt.Errorf("load user role assignments: %w", err)
	}
	for _, assignment := range assignments {
		index := indexes[assignment.UserID]
		users[index].RoleAssignments = append(users[index].RoleAssignments, assignment)
	}
	return nil
}
