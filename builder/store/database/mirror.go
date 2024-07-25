package database

import (
	"context"
	"fmt"
	"time"

	"opencsg.com/csghub-server/common/types"
)

type MirrorStore struct {
	db *DB
}

func NewMirrorStore() *MirrorStore {
	return &MirrorStore{
		db: defaultDB,
	}
}

type Mirror struct {
	ID             int64        `bun:",pk,autoincrement" json:"id"`
	Interval       string       `bun:",notnull" json:"interval"`
	SourceUrl      string       `bun:",notnull" json:"source_url"`
	MirrorSourceID int64        `bun:",notnull" json:"mirror_source_id"`
	MirrorSource   MirrorSource `bun:"rel:belongs-to,join:mirror_source_id=id" json:"mirror_source"`
	//source user name
	Username string `bun:",nullzero" json:"-"`
	// source access token
	AccessToken       string                 `bun:",nullzero" json:"-"`
	PushUrl           string                 `bun:",nullzero" json:"-"`
	PushUsername      string                 `bun:",nullzero" json:"-"`
	PushAccessToken   string                 `bun:",nullzero" json:"-"`
	RepositoryID      int64                  `bun:",notnull" json:"repository_id"`
	Repository        *Repository            `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt     time.Time              `bun:",nullzero" json:"last_updated_at"`
	SourceRepoPath    string                 `bun:",nullzero" json:"source_repo_path"`
	LocalRepoPath     string                 `bun:",nullzero" json:"local_repo_path"`
	LastMessage       string                 `bun:",nullzero" json:"last_message"`
	MirrorTaskID      int64                  `bun:",nullzero" json:"mirror_task_id"`
	PushMirrorCreated bool                   `bun:",nullzero,default:false" json:"push_mirror_created"`
	Status            types.MirrorTaskStatus `bun:",nullzero" json:"status"`
	Progress          int8                   `bun:",nullzero" json:"progress"`

	times
}

func (s *MirrorStore) IsExist(ctx context.Context, repoID int64) (exists bool, err error) {
	var mirror Mirror
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&mirror).
		Where("repository_id=?", repoID).
		Exists(ctx)
	return
}

func (s *MirrorStore) FindByRepoID(ctx context.Context, repoID int64) (*Mirror, error) {
	var mirror Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirror).
		Where("repository_id=?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mirror, nil
}

func (s *MirrorStore) FindByRepoPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Mirror, error) {
	var mirror Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirror).
		Join("JOIN repositories AS r ON mirror.repository_id = r.id ").
		Where("r.git_path=?", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mirror, nil
}

func (s *MirrorStore) FindWithMapping(ctx context.Context, repoType types.RepositoryType, namespace, name string, mapping types.Mapping) (*Mirror, error) {
	var mirror Mirror
	var err error
	if mapping == types.CSGHubMapping {
		return s.FindByRepoPath(ctx, repoType, namespace, name)
	} else if mapping == types.HFMapping {
		err = s.db.Operator.Core.NewSelect().
			Model(&mirror).
			Relation("Repository").
			Where("mirror.source_repo_path=?", fmt.Sprintf("%s/%s", namespace, name)).
			Where("repository.repository_type=?", repoType).
			Scan(ctx)
	} else {
		// auto mapping
		err = s.db.Operator.Core.NewSelect().
			Model(&mirror).
			Relation("Repository").
			Where("repository.git_path=? OR mirror.source_repo_path=?", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name), fmt.Sprintf("%s/%s", namespace, name)).
			Where("repository.repository_type=?", repoType).
			Scan(ctx)
	}
	if err != nil {
		return nil, err
	}
	return &mirror, nil
}

func (s *MirrorStore) Create(ctx context.Context, mirror *Mirror) (*Mirror, error) {
	err := s.db.Operator.Core.NewInsert().
		Model(mirror).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirror, nil
}

func (s *MirrorStore) Index(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *MirrorStore) IndexWithRepository(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repositoy").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *MirrorStore) NoPushMirror(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Where("push_mirror_created = ?", false).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *MirrorStore) PushedMirror(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("push_mirror_created = ?", true).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *MirrorStore) Update(ctx context.Context, mirror *Mirror) (err error) {
	err = assertAffectedOneRow(s.db.Operator.Core.NewUpdate().
		Model(mirror).
		WherePK().
		Exec(ctx),
	)

	return
}

func (s *MirrorStore) Delete(ctx context.Context, mirror *Mirror) (err error) {
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(mirror).
		WherePK().
		Exec(ctx)
	return
}

func (s *MirrorStore) Unfinished(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("status != ? OR status IS NULL", types.MirrorFinished).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *MirrorStore) Finished(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("status = ?", types.MirrorFinished).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}
