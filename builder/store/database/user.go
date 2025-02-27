package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

type UserStore interface {
	Index(ctx context.Context) (users []User, err error)
	IndexWithSearch(ctx context.Context, search string, per, page int) (users []User, count int, err error)
	FindByUsername(ctx context.Context, username string) (user User, err error)
	FindByID(ctx context.Context, id int) (user User, err error)
	FindByEmail(ctx context.Context, email string) (User, error)
	// Update write the user data back to db. odlUserName should not be empty if username changed
	Update(ctx context.Context, user *User, oldUserName string) (err error)
	ChangeUserName(ctx context.Context, username string, newUsername string) (err error)
	Create(ctx context.Context, user *User, namespace *Namespace) (err error)
	IsExist(ctx context.Context, username string) (exists bool, err error)
	IsExistByUUID(ctx context.Context, uuid string) (exists bool, err error)
	// FindByAccessToken retrieves user information based on the access token. The access token must be active and not expired.
	FindByAccessToken(ctx context.Context, token string) (*User, error)
	FindByGitAccessToken(ctx context.Context, token string) (*User, error)
	FindByUUID(ctx context.Context, uuid string) (*User, error)
	DeleteUserAndRelations(ctx context.Context, input User) (err error)
	CountUsers(ctx context.Context) (int, error)
}

// Implement the UserStore interface in UserStoreImpl
type userStoreImpl struct {
	db *DB
}

func NewUserStore() UserStore {
	return &userStoreImpl{
		db: defaultDB,
	}
}

func NewUserStoreWithDB(db *DB) UserStore {
	return &userStoreImpl{
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

	times
}

func (u *User) Roles() []string {
	if len(u.RoleMask) == 0 {
		return []string{}
	}
	return strings.Split(u.RoleMask, ",")
}

// CanAdmin checks if the user has admin or super_user roles.
//
// It returns a boolean indicating whether the user has admin or super_user roles.
func (u *User) CanAdmin() bool {
	return strings.Contains(u.RoleMask, "admin") || strings.Contains(u.RoleMask, "super_user")
}

func (u *User) SetRoles(roles []string) {
	u.RoleMask = strings.Join(roles, ",")
}

func (s *userStoreImpl) Index(ctx context.Context) (users []User, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&users).Scan(ctx, &users)
	if err != nil {
		return
	}
	return
}

func (s *userStoreImpl) IndexWithSearch(ctx context.Context, search string, per, page int) (users []User, count int, err error) {
	search = strings.ToLower(search)
	query := s.db.Operator.Core.NewSelect().
		Model(&users)
	if search != "" {
		query.Where("LOWER(username) like ? OR LOWER(email) like ?", fmt.Sprintf("%%%s%%", search), fmt.Sprintf("%%%s%%", search))
	}
	count, err = query.Count(ctx)
	if err != nil {
		return
	}
	query.Order("id asc").Limit(per).Offset((page - 1) * per)
	err = query.Scan(ctx, &users)
	if err != nil {
		return
	}
	return
}

func (s *userStoreImpl) FindByUsername(ctx context.Context, username string) (user User, err error) {
	user.Username = username
	err = s.db.Operator.Core.NewSelect().Model(&user).Where("username = ?", username).Scan(ctx)
	return
}

func (s *userStoreImpl) FindByEmail(ctx context.Context, email string) (user User, err error) {
	user.Email = email
	err = s.db.Operator.Core.NewSelect().Model(&user).Where("email = ?", email).Scan(ctx)
	return
}

func (s *userStoreImpl) FindByID(ctx context.Context, id int) (user User, err error) {
	user.ID = int64(id)
	err = s.db.Operator.Core.NewSelect().Model(&user).WherePK().Scan(ctx)
	return
}

func (s *userStoreImpl) Update(ctx context.Context, user *User, oldUserName string) (err error) {
	return s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
}

func (s *userStoreImpl) ChangeUserName(ctx context.Context, username string, newUsername string) (err error) {
	return s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
}

func (s *userStoreImpl) Create(ctx context.Context, user *User, namespace *Namespace) (err error) {
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
	return
}

func (s *userStoreImpl) IsExist(ctx context.Context, username string) (exists bool, err error) {
	return s.db.Operator.Core.
		NewSelect().
		Model((*User)(nil)).
		Where("username =?", username).
		Exists(ctx)
}

func (s *userStoreImpl) IsExistByUUID(ctx context.Context, uuid string) (exists bool, err error) {
	return s.db.Operator.Core.
		NewSelect().
		Model((*User)(nil)).
		Where("uuid =?", uuid).
		Exists(ctx)
}

// FindByAccessToken retrieves user information based on the access token. The access token must be active and not expired.
func (s *userStoreImpl) FindByAccessToken(ctx context.Context, token string) (*User, error) {
	var user User
	_, err := s.db.Operator.Core.
		NewSelect().
		ColumnExpr("u.*").
		TableExpr("users AS u").
		Join("JOIN access_tokens AS t ON u.id = t.user_id").
		Where("t.token = ? and t.is_active = true and (t.expired_at is null or t.expired_at > now()) ", token).
		Exec(ctx, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *userStoreImpl) FindByGitAccessToken(ctx context.Context, token string) (*User, error) {
	var user User
	_, err := s.db.Operator.Core.
		NewSelect().
		ColumnExpr("u.*").
		TableExpr("users AS u").
		Join("JOIN access_tokens AS t ON u.id = t.user_id").
		Where("t.token = ? and t.is_active = true and (t.expired_at is null or t.expired_at > now()) and app = 'git'", token).
		Exec(ctx, &user)

	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *userStoreImpl) FindByUUID(ctx context.Context, uuid string) (*User, error) {
	var user User
	err := s.db.Operator.Core.NewSelect().Model(&user).Where("uuid = ?", uuid).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *userStoreImpl) DeleteUserAndRelations(ctx context.Context, input User) (err error) {
	exists, err := s.IsExist(ctx, input.Username)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %v", err)
	}
	if !exists {
		return fmt.Errorf("user does not exist")
	}

	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Delete user
		if err = assertAffectedOneRow(tx.NewDelete().Model(&input).Where("id = ?", input.ID).Exec(ctx)); err != nil {
			return fmt.Errorf("failed to delete user %d: %v", input.ID, err)
		}
		// Get user's repository_ids
		var repoIDs []int64
		if err := s.db.Operator.Core.NewSelect().Column("id").Model(&Repository{}).Where("user_id = ?", input.ID).Scan(ctx, &repoIDs); err != nil {
			return fmt.Errorf("failed to get user repo ids: %v", err)
		}
		// Delete user's model
		if _, err := tx.NewDelete().Model(&Model{}).Where("repository_id IN (?)", bun.In(repoIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user models for user ID %d: %v", input.ID, err)
		}
		// Delete user's dataset
		if _, err := tx.NewDelete().Model(&Dataset{}).Where("repository_id IN (?)", bun.In(repoIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user datasets for user ID %d: %v", input.ID, err)
		}
		// Delete user's code
		if _, err := tx.NewDelete().Model(&Code{}).Where("repository_id IN (?)", bun.In(repoIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user codes for user ID %d: %v", input.ID, err)
		}
		// Delete user's space
		if _, err := tx.NewDelete().Model(&Space{}).Where("repository_id IN (?)", bun.In(repoIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user spaces for user ID %d: %v", input.ID, err)
		}
		// Delete user's namespace
		if _, err := tx.NewDelete().Model(&Namespace{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user namespace for user ID %d:  %v", input.ID, err)
		}
		// Delete user's prompts
		if _, err := tx.NewDelete().Model(&Prompt{}).Where("repository_id IN  (?)", bun.In(repoIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user prompts for user ID  %d:  %v", input.ID, err)
		}
		// Delete user's repositories runtime frameworks
		if _, err := tx.NewDelete().Model(&RepositoriesRuntimeFramework{}).Where("repo_id IN (?)", bun.In(repoIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user repositories runtime frameworks for user ID %d:  %v", input.ID, err)
		}
		// Delete user's repo
		if _, err = tx.NewDelete().Model(&Repository{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user repos for user ID %d: %v", input.ID, err)
		}
		// Delete user's access token
		if _, err := tx.NewDelete().Model(&AccessToken{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user access tokens for user ID %d: %v", input.ID, err)
		}
		// Delete user's organization
		if _, err := tx.NewDelete().Model(&Organization{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user organizations for user ID %d: %v", input.ID, err)
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
		// Delete user's prompt conversations
		if _, err := tx.NewDelete().Model(&PromptConversation{}).Where("user_id = ?", input.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user prompt conversations for user ID %d:  %v", input.ID, err)
		}

		return nil
	})
	return
}

func (s *userStoreImpl) CountUsers(ctx context.Context) (int, error) {
	var users []User
	q := s.db.Operator.Core.NewSelect().Model(&users)
	count, err := q.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}
