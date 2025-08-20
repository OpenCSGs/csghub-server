package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type Mirror struct {
	ID             int64        `bun:",pk,autoincrement" json:"id"`
	Interval       string       `bun:",notnull" json:"interval"`
	SourceUrl      string       `bun:",notnull" json:"source_url"`
	MirrorSourceID int64        `bun:",notnull" json:"mirror_source_id"`
	MirrorSource   MirrorSource `bun:"rel:belongs-to,join:mirror_source_id=id" json:"mirror_source"`
	//source user name
	Username string `bun:",nullzero" json:"-"`
	// source access token
	AccessToken            string                 `bun:",nullzero" json:"-"`
	PushUrl                string                 `bun:",nullzero" json:"-"`
	PushUsername           string                 `bun:",nullzero" json:"-"`
	PushAccessToken        string                 `bun:",nullzero" json:"-"`
	RepositoryID           int64                  `bun:",notnull" json:"repository_id"`
	Repository             *Repository            `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt          time.Time              `bun:",nullzero" json:"last_updated_at"`
	SourceRepoPath         string                 `bun:",nullzero" json:"source_repo_path"`
	LocalRepoPath          string                 `bun:",nullzero" json:"local_repo_path"`
	LastMessage            string                 `bun:",nullzero" json:"last_message"`
	MirrorTaskID           int64                  `bun:",nullzero" json:"mirror_task_id"`
	MirrorTasks            []*MirrorTask          `bun:"rel:has-many,join:mirror_task_id=id" json:"mirror_task"`
	PushMirrorCreated      bool                   `bun:",nullzero,default:false" json:"push_mirror_created"`
	Status                 types.MirrorTaskStatus `bun:",nullzero" json:"status"`
	Progress               int8                   `bun:",nullzero" json:"progress"`
	NextExecutionTimestamp time.Time              `bun:",nullzero" json:"next_execution_timestamp"`
	Priority               types.MirrorPriority   `bun:"mirror_priority,notnull,default:0" json:"priority"`
	RetryCount             int                    `bun:",nullzero" json:"retry_count"`
	RemoteUpdatedAt        time.Time              `bun:",nullzero" json:"remote_updated_at"`
	CurrentTaskID          int64                  `bun:",nullzero" json:"current_task_id"`
	CurrentTask            *MirrorTask            `bun:"rel:has-one,join:current_task_id=id" json:"current_task"`

	times
}

type MirrorSource struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	SourceName string `bun:",notnull,unique" json:"source_name"`
	InfoAPIUrl string `bun:",nullzero" json:"info_api_url"`

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, Mirror{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*Mirror)(nil)).
			Index("idx_mirrors_repository_id").
			Column("repository_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Mirror{})
	})

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, MirrorSource{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, MirrorSource{})
	})
}
