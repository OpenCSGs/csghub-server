package database

import (
	"context"

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
	ID           int64         `bun:",pk,autoincrement" json:"id"`
	GitID        int64         `bun:",notnull" json:"git_id"`
	Name         string        `bun:",notnull" json:"name"`
	Username     string        `bun:",notnull,unique" json:"username"`
	Email        string        `bun:",notnull,unique" json:"email"`
	Password     string        `bun:",notnull" json:"-"`
	AccessTokens []AccessToken `bun:"rel:has-many,join:id=user_id"`
	Namespaces   []Namespace   `bun:"rel:has-many,join:id=user_id" json:"namespace"`
	//TODO:add unique index after migration
	UUID string `bun:"," json:"uuid"`
	// user registered from default login page, from casdoor, etc. Possible values:
	//
	// - "default"
	// - "casdoor"
	RegProvider string `bun:"," json:"reg_provider"`
	times
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

func (s *UserStore) UpdateByUsername(ctx context.Context, u *User) (err error) {
	err = assertAffectedOneRow(s.db.Operator.Core.NewUpdate().
		Model(u).
		Where("username = ?", u.Username).
		Exec(ctx),
	)
	return
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
	var user User
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&user).
		Where("username =?", username).
		Exists(ctx)
	if err != nil {
		return
	}
	return
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

func (s *UserStore) FindByUUID(ctx context.Context, uuid string) (user User, err error) {
	user.UUID = uuid
	err = s.db.Operator.Core.NewSelect().Model(&user).Where("uuid = ?", uuid).Scan(ctx)
	return
}
