package database

import "context"

type GitServerAccessTokenStore struct {
	db *DB
}

func NewGitServerAccessTokenStore() *GitServerAccessTokenStore {
	return &GitServerAccessTokenStore{
		db: defaultDB,
	}
}

type GitServerAccessToken struct {
	ID    int64  `bun:",pk,autoincrement" json:"id"`
	Token string `bun:",notnull" json:"token"`
	times
}

func (s *GitServerAccessTokenStore) Create(ctx context.Context, gToken *GitServerAccessToken) (*GitServerAccessToken, error) {
	err := s.db.Operator.Core.NewInsert().
		Model(gToken).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return gToken, nil
}

func (s *GitServerAccessTokenStore) Index(ctx context.Context) ([]GitServerAccessToken, error) {
	var gTokens []GitServerAccessToken
	err := s.db.Operator.Core.NewSelect().
		Model(&gTokens).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return gTokens, nil
}
