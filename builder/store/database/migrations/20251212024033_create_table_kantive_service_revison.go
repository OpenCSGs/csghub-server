package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type KnativeServiceRevision struct {
	ID             int64  `bun:",pk,autoincrement" json:"id"`
	CommitID       string `json:"commit_id,omitempty"`
	SvcName        string `bun:",notnull" json:"svc_name"`
	RevisionName   string `json:"revision_name,omitempty"`
	TrafficPercent int64  `json:"traffic_percent,omitempty"`
	IsReady        bool
	Message        string `json:"message"`
	Reason         string `json:"reason"`

	CreateTime time.Time
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &KnativeServiceRevision{})
		if err != nil {
			return err
		}

		_, err = db.NewCreateIndex().Model(&KnativeServiceRevision{}).
			Index("idx_knative_service_revision_revision_name").
			Unique().
			Column("commit_id").
			Column("svc_name").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &KnativeServiceRevision{})
	})
}
