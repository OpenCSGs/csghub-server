package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// Define the UserStore interface
type UserStore interface {
	Index(ctx context.Context) ([]User, error)
	IndexWithSearch(ctx context.Context, search, verifyStatus string, labels []string, per, page int) ([]User, int, error)
	FindByUsername(ctx context.Context, username string) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	// Update write the user data back to db. odlUserName should not be empty if username changed
	Update(ctx context.Context, user *User, oldUserName string) error
	ChangeUserName(ctx context.Context, username string, newUsername string) error
	Create(ctx context.Context, user *User, namespace *Namespace) error
	IsExist(ctx context.Context, username string) (bool, error)
	IsExistByUUID(ctx context.Context, uuid string) (bool, error)
	FindByGitAccessToken(ctx context.Context, token string) (*User, error)
	FindByUUID(ctx context.Context, uuid string) (*User, error)
	DeleteUserAndRelations(ctx context.Context, input User, req types.CloseAccountReq) error
	CountUsers(ctx context.Context) (int, error)
	UpdateVerifyStatus(ctx context.Context, uuid string, status types.VerifyStatus) error
	UpdateLabels(ctx context.Context, uuid string, labels []string) error
	FindByUUIDs(ctx context.Context, uuids []string) ([]*User, error)
	SoftDeleteUserAndRelations(ctx context.Context, input User, req types.CloseAccountReq) (err error)
	IndexWithDeleted(ctx context.Context) (users []User, err error)
	FindByUsernameWithDeleted(ctx context.Context, username string) (User, error)
	IsExistWithDeleted(ctx context.Context, username string) (bool, error)
	GetEmails(ctx context.Context, per, page int) ([]string, int, error)
	GetUserUUIDs(ctx context.Context, per, page int) ([]string, int, error)
	UpdatePhone(ctx context.Context, userID int64, phone string, phoneArea string) error
}

// Implement the UserStore interface in UserStoreImpl
type UserStoreImpl struct {
	db *DB
}

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

func (u *User) Roles() []string {
	if len(u.RoleMask) == 0 {
		return []string{}
	}
	return strings.Split(u.RoleMask, ",")
}

// CanAdmin checks if the user has admin or super_user roles.
// It returns a boolean indicating whether the user has admin or super_user roles.
func (u *User) CanAdmin() bool {
	return strings.Contains(u.RoleMask, "admin") || strings.Contains(u.RoleMask, "super_user")
}

func (u *User) SetRoles(roles []string) {
	u.RoleMask = strings.Join(roles, ",")
}

func (s *UserStoreImpl) Index(ctx context.Context) (users []User, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&users).Scan(ctx, &users)
	return users, errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) IndexWithSearch(ctx context.Context, search, verifyStatus string, labels []string, per, page int) (users []User, count int, err error) {
	search = strings.ToLower(search)
	query := s.db.Operator.Core.NewSelect().
		Model(&users)
	if search != "" {
		query.Where("LOWER(username) like ? OR LOWER(email) like ? OR phone like ?", fmt.Sprintf("%%%s%%", search), fmt.Sprintf("%%%s%%", search), fmt.Sprintf("%%%s%%", search))
	}
	if verifyStatus != "" {
		query.Where("verify_status = ?", verifyStatus)
	}
	if len(labels) != 0 {
		labelsJSON, err := json.Marshal(labels)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal labels: %w, %w", err, errorx.ErrInternalServerError)
		}
		query.Where("labels @> ?", string(labelsJSON))
	}
	count, err = query.Count(ctx)
	if err != nil {
		return users, count, errorx.HandleDBError(err, nil)
	}
	query.Order("id asc").Limit(per).Offset((page - 1) * per)
	err = query.Scan(ctx, &users)
	return users, count, errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) FindByUsername(ctx context.Context, username string) (user User, err error) {
	user.Username = username
	err = s.db.Operator.Core.NewSelect().
		Model(&user).
		Where("username = ?", username).
		Scan(ctx)
	return user, errorx.HandleDBError(err, map[string]interface{}{"username": username})
}

func (s *UserStoreImpl) FindByEmail(ctx context.Context, email string) (user User, err error) {
	user.Email = email
	err = s.db.Operator.Core.NewSelect().Model(&user).Where("email = ?", email).Scan(ctx)
	return user, errorx.HandleDBError(err, nil)
}

func (s *UserStoreImpl) FindByID(ctx context.Context, id int) (user User, err error) {
	user.ID = int64(id)
	err = s.db.Operator.Core.NewSelect().Model(&user).WherePK().Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return
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

// FindByAccessToken retrieves user information based on the access token. The access token must be active and not expired.
func (s *UserStoreImpl) FindByGitAccessToken(ctx context.Context, token string) (*User, error) {
	var user User
	_, err := s.db.Operator.Core.
		NewSelect().
		ColumnExpr("u.*").
		TableExpr("users AS u").
		Join("JOIN access_tokens AS t ON u.id = t.user_id").
		Where("t.token = ? and t.is_active = true and (t.expired_at is null or t.expired_at > now()) and app = 'git'", token).
		Exec(ctx, &user)

	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &user, nil
}

func (s *UserStoreImpl) FindByUUID(ctx context.Context, uuid string) (*User, error) {
	var user User
	err := s.db.Operator.Core.NewSelect().
		Model(&user).
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

func (s *UserStoreImpl) DeleteUserAndRelations(ctx context.Context, input User, req types.CloseAccountReq) (err error) {
	exists, err := s.IsExistWithDeleted(ctx, input.Username)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %v", err)
	}
	if !exists {
		return errorx.ErrDatabaseNoRows
	}

	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Delete user
		if err = assertAffectedOneRow(tx.NewDelete().Model(&input).Where("id = ?", input.ID).ForceDelete().Exec(ctx)); err != nil {
			return fmt.Errorf("failed to delete user %d: %v", input.ID, err)
		}

		if !req.Repository {
			// Get user's repository_ids
			var repoIDs []int64
			if err := s.db.Operator.Core.NewSelect().Column("id").Model(&Repository{}).Where("user_id = ?", input.ID).Scan(ctx, &repoIDs); err != nil {
				return fmt.Errorf("failed to get user repo ids: %v", err)
			}
			// Delete user's model
			if _, err := tx.NewDelete().Model(&Model{}).Where("repository_id IN (?)", bun.In(repoIDs)).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user models for user ID %d: %v", input.ID, err)
			}
			// Delete user's dataset
			if _, err := tx.NewDelete().Model(&Dataset{}).Where("repository_id IN (?)", bun.In(repoIDs)).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user datasets for user ID %d: %v", input.ID, err)
			}
			// Delete user's code
			if _, err := tx.NewDelete().Model(&Code{}).Where("repository_id IN (?)", bun.In(repoIDs)).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user codes for user ID %d: %v", input.ID, err)
			}
			// Delete user's space
			if _, err := tx.NewDelete().Model(&Space{}).Where("repository_id IN (?)", bun.In(repoIDs)).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user spaces for user ID %d: %v", input.ID, err)
			}
			// Delete user's namespace
			if _, err := tx.NewDelete().Model(&Namespace{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user namespace for user ID %d:  %v", input.ID, err)
			}
			// Delete user's prompts
			if _, err := tx.NewDelete().Model(&Prompt{}).Where("repository_id IN  (?)", bun.In(repoIDs)).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user prompts for user ID  %d:  %v", input.ID, err)
			}
			// Delete user's repositories runtime frameworks
			if _, err := tx.NewDelete().Model(&RepositoriesRuntimeFramework{}).Where("repo_id IN (?)", bun.In(repoIDs)).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user repositories runtime frameworks for user ID %d:  %v", input.ID, err)
			}
			// Delete user's mcp servers
			if _, err := tx.NewDelete().Model(&MCPServer{}).Where("repository_id IN (?)", bun.In(repoIDs)).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user mcp servers for user ID %d:  %v", input.ID, err)
			}
			// Delete user's repo
			if _, err = tx.NewDelete().Model(&Repository{}).Where("user_id = ?", input.ID).ForceDelete().Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete user repos for user ID %d: %v", input.ID, err)
			}
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

func (s *UserStoreImpl) CountUsers(ctx context.Context) (int, error) {
	var users []User
	q := s.db.Operator.Core.NewSelect().Model(&users)
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

func (s *UserStoreImpl) FindByUUIDs(ctx context.Context, uuids []string) ([]*User, error) {
	var users []*User
	if len(uuids) == 0 {
		return users, nil
	}

	err := s.db.Operator.Core.NewSelect().
		Model(&users).
		Where("uuid IN (?)", bun.In(uuids)).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return users, nil
}

func (s *UserStoreImpl) IndexWithDeleted(ctx context.Context) (users []User, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&users).WhereAllWithDeleted().Scan(ctx, &users)
	err = errorx.HandleDBError(err, nil)
	return
}

func (s *UserStoreImpl) SoftDeleteUserAndRelations(ctx context.Context, input User, req types.CloseAccountReq) (err error) {
	exists, err := s.IsExist(ctx, input.Username)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %w", err)
	}
	if !exists {
		return errorx.ErrDatabaseNoRows
	}
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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

func (s *UserStoreImpl) FindByUsernameWithDeleted(ctx context.Context, username string) (user User, err error) {
	user.Username = username
	err = s.db.Operator.Core.NewSelect().Model(&user).WhereAllWithDeleted().Where("username = ?", username).Scan(ctx)
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
