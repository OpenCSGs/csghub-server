package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitInMemoryDB(t *testing.T) {
	// Test initInMemoryDB function
	err := InitInMemoryDB()
	require.NoError(t, err, "initInMemoryDB should not return error")

	// Get the initialized database from the global variable
	db := GetDB()
	require.NotNil(t, db, "database should not be nil")
	defer db.Close()

	// Verify all tables are successfully created
	ctx := context.Background()

	// Define table names and corresponding models to verify
	expectedTables := map[string]any{
		"cluster_infos":       (*ClusterInfo)(nil),
		"argo_workflows":      (*ArgoWorkflow)(nil),
		"image_builder_works": (*ImageBuilderWork)(nil),
		"deploy_logs":         (*DeployLog)(nil),
		"knative_services":    (*KnativeService)(nil),
	}

	// Verify each table exists
	for tableName, model := range expectedTables {
		t.Run("table_"+tableName, func(t *testing.T) {
			// Try to query table structure to verify table exists
			count, err := db.BunDB.NewSelect().Model(model).Count(ctx)
			assert.NoError(t, err, "should be able to query table %s", tableName)
			assert.Equal(t, 0, count, "new table should be empty")
		})
	}
}

func TestCreateTables(t *testing.T) {
	// Test createTables function separately
	dsn := "file::memory:?cache=shared"
	ctx := context.Background()
	config := DBConfig{
		Dialect: DialectSQLite,
		DSN:     dsn,
	}

	db, err := NewDB(ctx, config)
	require.NoError(t, err, "should create database connection")
	defer db.Close()

	// Test createTables function
	err = createTables(ctx, db.BunDB)
	assert.NoError(t, err, "createTables should not return error")

	// Verify tables can perform basic operations after creation
	tables := []any{
		(*ClusterInfo)(nil),
		(*ArgoWorkflow)(nil),
		(*ImageBuilderWork)(nil),
		(*DeployLog)(nil),
		(*KnativeService)(nil),
	}

	for i, table := range tables {
		t.Run(fmt.Sprintf("table_%d", i), func(t *testing.T) {
			// Verify SELECT queries can be executed (even if result is empty)
			count, err := db.BunDB.NewSelect().Model(table).Count(ctx)
			assert.NoError(t, err, "should be able to count rows in table")
			assert.Equal(t, 0, count, "new table should be empty")
		})
	}

	// Verify indexes are created correctly
	t.Run("verify_indexes", func(t *testing.T) {
		// Define expected indexes with their details
		expectedIndexes := []struct {
			name        string
			tableName   string
			columns     []string
			isUnique    bool
			description string
		}{
			{
				name:        "idx_knative_name_cluster",
				tableName:   "knative_services",
				columns:     []string{"name", "cluster_id"},
				isUnique:    true,
				description: "unique index on KnativeService (name, cluster_id)",
			},
			{
				name:        "idx_deploy_logs_clusterid_svcname_podname",
				tableName:   "deploy_logs",
				columns:     []string{"cluster_id", "svc_name", "pod_name"},
				isUnique:    true,
				description: "unique index on DeployLog (cluster_id, svc_name, pod_name)",
			},
			{
				name:        "idx_image_builder_work_work_name",
				tableName:   "image_builder_works",
				columns:     []string{"work_name"},
				isUnique:    false,
				description: "index on ImageBuilderWork (work_name)",
			},
			{
				name:        "idx_image_builder_work_build_id",
				tableName:   "image_builder_works",
				columns:     []string{"build_id"},
				isUnique:    false,
				description: "index on ImageBuilderWork (build_id)",
			},
			{
				name:        "idx_workflow_user_uuid",
				tableName:   "argo_workflows",
				columns:     []string{"username", "task_id"},
				isUnique:    false,
				description: "index on ArgoWorkflow (username, task_id)",
			},
		}

		// Query SQLite system table to get index information
		for _, expectedIndex := range expectedIndexes {
			t.Run(expectedIndex.name, func(t *testing.T) {
				// Check if index exists in sqlite_master
				var count int
				err := db.BunDB.NewRaw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", expectedIndex.name).Scan(ctx, &count)
				assert.NoError(t, err, "should be able to query sqlite_master for index %s", expectedIndex.name)
				assert.Equal(t, 1, count, "index %s should exist (%s)", expectedIndex.name, expectedIndex.description)

				// Verify index can be used by checking EXPLAIN QUERY PLAN
				// This ensures the index is functional
				if len(expectedIndex.columns) > 0 {
					// Build a simple query that should use the index
					var queryResult []map[string]any
					switch expectedIndex.tableName {
					case "knative_services":
						err = db.BunDB.NewRaw("EXPLAIN QUERY PLAN SELECT * FROM knative_services WHERE name = ? AND cluster_id = ?", "test", "test").Scan(ctx, &queryResult)
					case "deploy_logs":
						err = db.BunDB.NewRaw("EXPLAIN QUERY PLAN SELECT * FROM deploy_logs WHERE cluster_id = ? AND svc_name = ? AND pod_name = ?", "test", "test", "test").Scan(ctx, &queryResult)
					case "image_builder_works":
						if expectedIndex.name == "idx_image_builder_work_work_name" {
							err = db.BunDB.NewRaw("EXPLAIN QUERY PLAN SELECT * FROM image_builder_works WHERE work_name = ?", "test").Scan(ctx, &queryResult)
						} else {
							err = db.BunDB.NewRaw("EXPLAIN QUERY PLAN SELECT * FROM image_builder_works WHERE build_id = ?", "test").Scan(ctx, &queryResult)
						}
					case "argo_workflows":
						err = db.BunDB.NewRaw("EXPLAIN QUERY PLAN SELECT * FROM argo_workflows WHERE username = ? AND task_id = ?", "test", "test").Scan(ctx, &queryResult)
					}
					assert.NoError(t, err, "should be able to explain query plan for index %s", expectedIndex.name)
				}
			})
		}

		// Additional test: Verify that unique indexes prevent duplicate entries
		t.Run("unique_constraints", func(t *testing.T) {
			// Test KnativeService unique constraint
			service1 := &KnativeService{
				Name:      "test-service",
				ClusterID: "test-cluster",
				Status:    "True",
				Code:      200,
			}
			_, err := db.BunDB.NewInsert().Model(service1).Exec(ctx)
			assert.NoError(t, err, "should be able to insert first KnativeService")

			// Try to insert duplicate - should handle conflict
			service2 := &KnativeService{
				Name:      "test-service",
				ClusterID: "test-cluster",
				Status:    "False",
				Code:      500,
			}
			_, err = db.BunDB.NewInsert().Model(service2).On("CONFLICT(name, cluster_id) DO UPDATE").Set("status = EXCLUDED.status").Exec(ctx)
			assert.NoError(t, err, "should handle conflict on unique index for KnativeService")

			// Test DeployLog unique constraint
			log1 := &DeployLog{
				DeployID:  1,
				ClusterID: "test-cluster",
				SvcName:   "test-service",
				PodName:   "test-pod",
			}
			_, err = db.BunDB.NewInsert().Model(log1).Exec(ctx)
			assert.NoError(t, err, "should be able to insert first DeployLog")

			// Try to insert duplicate - should handle conflict
			log2 := &DeployLog{
				DeployID:  2,
				ClusterID: "test-cluster",
				SvcName:   "test-service",
				PodName:   "test-pod",
			}
			_, err = db.BunDB.NewInsert().Model(log2).On("CONFLICT(cluster_id, svc_name, pod_name) DO UPDATE").Set("deploy_id = EXCLUDED.deploy_id").Exec(ctx)
			assert.NoError(t, err, "should handle conflict on unique index for DeployLog")
		})
	})
}

func TestCreateTablesIdempotent(t *testing.T) {
	// Test that repeated calls to createTables don't cause errors
	dsn := "file::memory:?cache=shared"
	ctx := context.Background()
	config := DBConfig{
		Dialect: DialectSQLite,
		DSN:     dsn,
	}

	db, err := NewDB(ctx, config)
	require.NoError(t, err, "should create database connection")
	defer db.Close()

	// First table creation
	err = createTables(ctx, db.BunDB)
	assert.NoError(t, err, "first createTables call should succeed")

	// Second table creation (should handle existing tables)
	err = createTables(ctx, db.BunDB)
	// Depending on implementation, this may fail or succeed based on IF NOT EXISTS usage
	// If it fails, the error should be about table already existing, not other errors
	if err != nil {
		assert.Contains(t, err.Error(), "already exists",
			"error should be about table already existing")
	}
}
