package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
)

func InitInMemoryDB() error {
	dsn := "file::memory:?cache=shared"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config := DBConfig{
		Dialect: DialectSQLite,
		DSN:     dsn,
	}
	db, err := NewDB(ctx, config)
	if err != nil {
		return err
	}

	err = createTables(ctx, db.BunDB)
	if err != nil {
		return fmt.Errorf("failed to create table in memory db, %w", err)
	}

	defaultDB = db
	slog.Info("init memory db success")
	return nil
}

func createTables(ctx context.Context, db *bun.DB) error {
	tables := []interface{}{
		(*ClusterInfo)(nil),
		(*ArgoWorkflow)(nil),
		(*ImageBuilderWork)(nil),
		(*DeployLog)(nil),
		(*KnativeService)(nil),
		(*KnativeServiceRevision)(nil),
	}
	for _, table := range tables {
		_, err := db.NewCreateTable().Model(table).Exec(ctx)
		if err != nil {
			return err
		}
	}

	return createIndexes(ctx, db)
}

func createIndexes(ctx context.Context, db *bun.DB) error {
	_, err := db.NewCreateIndex().
		Model((*KnativeService)(nil)).
		Index("idx_knative_name_cluster").
		Column("name", "cluster_id").
		Unique().
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("fail to create index idx_knative_name_cluster_user : %w", err)
	}

	_, err = db.NewCreateIndex().
		Model(&DeployLog{}).
		Index("idx_deploy_logs_clusterid_svcname_podname").
		Unique().
		Column("cluster_id", "svc_name", "pod_name").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = db.NewCreateIndex().
		Model(&ImageBuilderWork{}).
		Index("idx_image_builder_work_work_name").
		Column("work_name").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = db.NewCreateIndex().
		Model(&ImageBuilderWork{}).
		Index("idx_image_builder_work_build_id").
		Column("build_id").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewCreateIndex().
		Model((*ArgoWorkflow)(nil)).
		Index("idx_workflow_user_uuid").
		Column("username", "task_id").
		Exec(ctx)
	return err

}
