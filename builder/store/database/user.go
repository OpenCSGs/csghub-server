package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

type UserStore struct {
	db *DB
}

func NewUserStore() *UserStore {
	return &UserStore{
		db: defaultDB,
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

func (s *UserStore) Index(ctx context.Context) (users []User, err error) {
	err = s.db.Operator.Core.NewSelect().Model(&users).Scan(ctx, &users)
	if err != nil {
		return
	}
	return
}

func (s *UserStore) FindByUsername(ctx context.Context, username string) (user User, err error) {
	user.Username = username
	err = s.db.Operator.Core.NewSelect().Model(&user).Where("username = ?", username).Scan(ctx)
	return
}

func (s *UserStore) FindByID(ctx context.Context, id int) (user User, err error) {
	user.ID = int64(id)
	err = s.db.Operator.Core.NewSelect().Model(&user).WherePK().Scan(ctx)
	return
}

func (s *UserStore) Update(ctx context.Context, user *User) (err error) {
	err = assertAffectedOneRow(s.db.Operator.Core.NewUpdate().
		Model(user).
		WherePK().
		Exec(ctx),
	)

	return
}

func (s *UserStore) ChangeUserName(ctx context.Context, username string, newUsername string) (err error) {
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

func (s *UserStore) Create(ctx context.Context, user *User, namespace *Namespace) (err error) {
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

func (s *UserStore) IsExist(ctx context.Context, username string) (exists bool, err error) {
	return s.db.Operator.Core.
		NewSelect().
		Model((*User)(nil)).
		Where("username =?", username).
		Exists(ctx)
}

func (s *UserStore) IsExistByUUID(ctx context.Context, uuid string) (exists bool, err error) {
	return s.db.Operator.Core.
		NewSelect().
		Model((*User)(nil)).
		Where("uuid =?", uuid).
		Exists(ctx)
}

// FindByAccessToken retrieves user information based on the access token. The access token must be active and not expired.
func (s *UserStore) FindByAccessToken(ctx context.Context, token string) (*User, error) {
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

func (s *UserStore) FindByGitAccessToken(ctx context.Context, token string) (*User, error) {
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

func (s *UserStore) FindByUUID(ctx context.Context, uuid string) (*User, error) {
	var user User
	err := s.db.Operator.Core.NewSelect().Model(&user).Where("uuid = ?", uuid).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) GetActiveUserCount(ctx context.Context) (int, error) {
	return s.db.Operator.Core.
		NewSelect().
		Model(&User{}).
		Count(ctx)
}
