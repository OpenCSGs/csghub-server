package database

import "context"

type MirrorTokenStore struct {
	db *DB
}

func NewMirrorTokenStore() *MirrorTokenStore {
	return &MirrorTokenStore{
		db: defaultDB,
	}
}

type MirrorToken struct {
	ID              int64  `bun:",pk,autoincrement" json:"id"`
	Token           string `bun:",notnull" json:"token"`
	ConcurrentCount int    `bun:",nullzero" json:"concurrent_count"`
	MaxBandwidth    int    `bun:",nullzero" json:"max_bandwidth"`
	times
}

func (s *MirrorTokenStore) Create(ctx context.Context, mirrorToken *MirrorToken) (*MirrorToken, error) {
	err := s.db.Operator.Core.NewInsert().
		Model(mirrorToken).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrorToken, nil
}

func (s *MirrorTokenStore) MirrorTokenExists(ctx context.Context) (bool, error) {
	return s.db.Operator.Core.NewSelect().
		Model((*MirrorToken)(nil)).
		Exists(ctx)
}

func (s *MirrorTokenStore) DeleteAll(ctx context.Context) error {
	_, err := s.db.Operator.Core.NewDelete().Model((*MirrorToken)(nil)).Where("1=1").Exec(ctx)
	return err
}

func (s *MirrorTokenStore) First(ctx context.Context) (*MirrorToken, error) {
	var mt MirrorToken
	err := s.db.Operator.Core.NewSelect().
		Model(&mt).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mt, nil
}
