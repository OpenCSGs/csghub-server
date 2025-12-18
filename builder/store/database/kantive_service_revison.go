package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type KnativeServiceRevisionStore interface {
	QueryRevision(ctx context.Context, svcName, commitID string) (*KnativeServiceRevision, error)
	AddRevision(ctx context.Context, revision KnativeServiceRevision) error
	ListRevisions(ctx context.Context, SvcName string) ([]KnativeServiceRevision, error)
	DeleteRevision(ctx context.Context, svcName, commitID string) error
}
type KnativeServiceRevision struct {
	ID             int64  `bun:",pk,autoincrement" json:"id"`
	CommitID       string `json:"commit_id"`
	SvcName        string `bun:",notnull" json:"svc_name"`
	RevisionName   string `json:"revision_name"`
	TrafficPercent int64  `json:"traffic_percent"`
	IsReady        bool   `json:"is_ready"`
	Message        string `json:"message"`
	Reason         string `json:"reason"`

	CreateTime time.Time
}

type KnativeServiceRevisionImpl struct {
	db *DB
}

func NewKnativeServiceRevisionStore() KnativeServiceRevisionStore {
	return &KnativeServiceRevisionImpl{
		db: defaultDB,
	}
}

func NewKnativeServiceRevisionStoreWithDB(db *DB) KnativeServiceRevisionStore {
	return &KnativeServiceRevisionImpl{
		db: db,
	}
}

func (k *KnativeServiceRevisionImpl) AddRevision(ctx context.Context, revision KnativeServiceRevision) error {
	_, err := k.db.Operator.Core.NewInsert().
		Model(&revision).
		On("CONFLICT(svc_name,commit_id) DO UPDATE").
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (k *KnativeServiceRevisionImpl) ListRevisions(ctx context.Context, SvcName string) ([]KnativeServiceRevision, error) {
	var revisions []KnativeServiceRevision
	err := k.db.Operator.Core.NewSelect().Model(&revisions).Where("svc_name = ?", SvcName).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return revisions, nil
}

func (k *KnativeServiceRevisionImpl) QueryRevision(ctx context.Context, svcName, commitID string) (*KnativeServiceRevision, error) {
	var revision KnativeServiceRevision
	err := k.db.Operator.Core.NewSelect().Model(&revision).Where("svc_name = ? AND commit_id = ?", svcName, commitID).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &revision, nil
}

func (k *KnativeServiceRevisionImpl) DeleteRevision(ctx context.Context, svcName, commitID string) error {
	_, err := k.db.Operator.Core.NewDelete().Model(&KnativeServiceRevision{}).Where("svc_name = ? AND commit_id = ?", svcName, commitID).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}
