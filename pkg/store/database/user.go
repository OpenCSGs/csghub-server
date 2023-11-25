package database

import (
	"context"
	"time"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
)

type UserStore struct {
	db *model.DB
}

func NewUserStore(db *model.DB) *UserStore {
	return &UserStore{
		db: db,
	}
}

type User struct {
	ID       int    `bun:",pk,autoincrement" json:"id"`
	GitID    int    `bun:",notnull" json:"git_id"`
	Name     string `bun:",notnull" json:"name"`
	Username string `bun:",notnull,unique" json:"username"`
	Email    string `bun:",notnull,unique" json:"email"`
	Password string `bun:",notnull" json:"-"`
	times
}

func (s *UserStore) FindByUsername(ctx context.Context, username string) (user User, err error) {
	user.Username = username
	err = s.db.Operator.Core.NewSelect().Model(&user).Where("username = ?", username).Scan(ctx)
	return
}

func (s *UserStore) FindByID(ctx context.Context, id int) (user User, err error) {
	user.ID = id
	err = s.db.Operator.Core.NewSelect().Model(&user).WherePK().Scan(ctx)
	return
}

func (s *UserStore) UpdateByUsername(ctx context.Context, u *User) (err error) {
	u.UpdatedAt = time.Now()
	err = assertAffectedOneRow(s.db.Operator.Core.NewUpdate().
		Model(u).
		Where("username = ?", u.Username).
		Exec(ctx),
	)
	return
}

func (s *UserStore) CreateUser(ctx context.Context, user *User) (err error) {
	err = s.db.Operator.Core.NewInsert().Model(user).Scan(ctx)
	return
}
