package database

import "context"

type gitServerAccessTokenStoreImpl struct {
	db *DB
}

type GitServerAccessTokenStore interface {
	Create(ctx context.Context, gToken *GitServerAccessToken) (*GitServerAccessToken, error)
	Index(ctx context.Context) ([]GitServerAccessToken, error)
	FindByType(ctx context.Context, serverType string) ([]GitServerAccessToken, error)
}

func NewGitServerAccessTokenStore() GitServerAccessTokenStore {
	return &gitServerAccessTokenStoreImpl{
		db: defaultDB,
	}
}

type GitServerType string

const (
	MirrorServer GitServerType = "mirror"
	GitServer    GitServerType = "git"
)

type GitServerAccessToken struct {
	ID         int64         `bun:",pk,autoincrement" json:"id"`
	Token      string        `bun:",notnull" json:"token"`
	ServerType GitServerType `bun:",notnull" json:"server_type"`
	times
}

func (s *gitServerAccessTokenStoreImpl) Create(ctx context.Context, gToken *GitServerAccessToken) (*GitServerAccessToken, error) {
	err := s.db.Operator.Core.NewInsert().
		Model(gToken).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return gToken, nil
}

func (s *gitServerAccessTokenStoreImpl) Index(ctx context.Context) ([]GitServerAccessToken, error) {
	var gTokens []GitServerAccessToken
	err := s.db.Operator.Core.NewSelect().
		Model(&gTokens).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return gTokens, nil
}

func (s *gitServerAccessTokenStoreImpl) FindByType(ctx context.Context, serverType string) ([]GitServerAccessToken, error) {
	var gTokens []GitServerAccessToken
	err := s.db.Operator.Core.NewSelect().
		Model(&gTokens).
		Where("server_type = ?", serverType).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return gTokens, nil
}
