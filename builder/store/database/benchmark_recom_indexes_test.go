package database_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

const (
	TestDataSize       = 500000 // 500,000 records (sufficient to test index performance)
	BenchmarkRepoCount = 10000  // Simulate 10,000 repositories
)

// BenchmarkRecomIndexes_CoveringIndex tests covering index performance
func BenchmarkRecomIndexes_CoveringIndex(b *testing.B) {
	db := tests.InitTransactionTestDB()
	defer db.Close()

	insertTestData(b, db)
	createCoveringIndex(b, db)

	b.ResetTimer()
	b.Run("SortQuery", func(b *testing.B) {
		benchmarkSortQuery(b, db)
	})

	b.Run("CombinedQuery", func(b *testing.B) {
		benchmarkCombinedQuery(b, db)
	})

	b.Run("CombinedQuery_Where", func(b *testing.B) {
		benchmarkCombinedQuery_Where(b, db)
	})
}

// BenchmarkRecomIndexes_SpecializedIndex tests specialized index performance
func BenchmarkRecomIndexes_SpecializedIndex(b *testing.B) {
	db := tests.InitTransactionTestDB()
	defer db.Close()

	insertTestData(b, db)
	createSpecializedIndex(b, db)

	b.ResetTimer()
	b.Run("SortQuery", func(b *testing.B) {
		benchmarkSortQuery(b, db)
	})

	b.Run("CombinedQuery", func(b *testing.B) {
		benchmarkCombinedQuery(b, db)
	})

	b.Run("CombinedQuery_Where", func(b *testing.B) {
		benchmarkCombinedQuery_Where(b, db)
	})
}

// BenchmarkRecomIndexes_NoIndex tests performance without an index (as a baseline comparison)
func BenchmarkRecomIndexes_NoIndex(b *testing.B) {
	db := tests.InitTransactionTestDB()
	defer db.Close()

	insertTestData(b, db)
	dropAllIndexes(b, db)

	b.ResetTimer()

	b.Run("SortQuery", func(b *testing.B) {
		benchmarkSortQuery(b, db)
	})

	b.Run("CombinedQuery", func(b *testing.B) {
		benchmarkCombinedQuery(b, db)
	})

	b.Run("CombinedQuery_Where", func(b *testing.B) {
		benchmarkCombinedQuery_Where(b, db)
	})
}

// insertTestData quickly inserts test data
func insertTestData(b *testing.B, db *database.DB) {
	ctx := context.Background()
	start := time.Now()

	// First, insert test users
	insertTestUsers(b, db, ctx)

	// Then, insert test repositories
	insertTestRepositories(b, db, ctx)

	// Finally, insert recommendation scores
	insertTestRecomScores(b, db, ctx)

	b.Logf("Test data insertion complete, total time: %v", time.Since(start))
}

// insertTestUsers inserts test user data
func insertTestUsers(b *testing.B, db *database.DB, ctx context.Context) {
	start := time.Now()

	var userBatch []*database.User
	for userID := int64(1); userID <= int64(BenchmarkRepoCount/100); userID++ { // Create fewer users than repos
		userBatch = append(userBatch, &database.User{
			ID:       userID,
			GitID:    userID + 1000,
			NickName: fmt.Sprintf("TestUser%d", userID),
			Username: fmt.Sprintf("testuser%d", userID),
			Email:    fmt.Sprintf("testuser%d@example.com", userID),
			Password: "test_password",
			UUID:     fmt.Sprintf("uuid-%d", userID),
		})

		if len(userBatch) >= 1000 {
			_, err := db.Operator.Core.NewInsert().Model(&userBatch).Exec(ctx)
			require.NoError(b, err)
			userBatch = userBatch[:0]
		}
	}

	if len(userBatch) > 0 {
		_, err := db.Operator.Core.NewInsert().Model(&userBatch).Exec(ctx)
		require.NoError(b, err)
	}

	b.Logf("User data insertion time: %v", time.Since(start))
}

// insertTestRepositories inserts test repository data
func insertTestRepositories(b *testing.B, db *database.DB, ctx context.Context) {
	start := time.Now()

	repoTypes := []types.RepositoryType{
		types.ModelRepo,
		types.DatasetRepo,
		types.SpaceRepo,
		types.CodeRepo,
		types.PromptRepo,
	}

	var repoBatch []*database.Repository
	for repoID := int64(1); repoID <= int64(BenchmarkRepoCount); repoID++ {
		userID := ((repoID - 1) % int64(BenchmarkRepoCount/100)) + 1 // Distribute repos among users
		repoType := repoTypes[(repoID-1)%int64(len(repoTypes))]

		repoBatch = append(repoBatch, &database.Repository{
			ID:             repoID,
			UserID:         userID,
			Path:           fmt.Sprintf("testuser%d/testrepo%d", userID, repoID),
			GitPath:        fmt.Sprintf("testuser%d/testrepo%d.git", userID, repoID),
			Name:           fmt.Sprintf("testrepo%d", repoID),
			Nickname:       fmt.Sprintf("Test Repository %d", repoID),
			Description:    fmt.Sprintf("Test repository %d for benchmark testing", repoID),
			Private:        rand.Intn(2) == 1, // Random private/public
			DefaultBranch:  "main",
			RepositoryType: repoType,
			HTTPCloneURL:   fmt.Sprintf("https://example.com/testuser%d/testrepo%d.git", userID, repoID),
			SSHCloneURL:    fmt.Sprintf("git@example.com:testuser%d/testrepo%d.git", userID, repoID),
			Source:         types.LocalSource,
			Likes:          rand.Int63n(1000),
			DownloadCount:  rand.Int63n(10000),
			StarCount:      rand.Intn(500),
		})

		if len(repoBatch) >= 1000 {
			_, err := db.Operator.Core.NewInsert().Model(&repoBatch).Exec(ctx)
			require.NoError(b, err)
			repoBatch = repoBatch[:0]
		}
	}

	if len(repoBatch) > 0 {
		_, err := db.Operator.Core.NewInsert().Model(&repoBatch).Exec(ctx)
		require.NoError(b, err)
	}

	b.Logf("Repository data insertion time: %v", time.Since(start))
}

// insertTestRecomScores inserts recommendation score data
func insertTestRecomScores(b *testing.B, db *database.DB, ctx context.Context) {
	start := time.Now()

	weightNames := []database.RecomWeightName{
		database.RecomWeightTotal,
		database.RecomWeightFreshness,
		database.RecomWeightDownloads,
		database.RecomWeightQuality,
		database.RecomWeightOp,
	}

	var batch []*database.RecomRepoScore
	count := 0

	for repoID := int64(1); repoID <= int64(BenchmarkRepoCount) && count < TestDataSize; repoID++ {
		for _, weightName := range weightNames {
			if count >= TestDataSize {
				break
			}

			batch = append(batch, &database.RecomRepoScore{
				RepositoryID: repoID,
				WeightName:   weightName,
				Score:        rand.Float64() * 100,
			})
			count++

			if len(batch) >= 1000 {
				_, err := db.Operator.Core.NewInsert().Model(&batch).Exec(ctx)
				require.NoError(b, err)
				batch = batch[:0]
			}
		}
	}

	if len(batch) > 0 {
		_, err := db.Operator.Core.NewInsert().Model(&batch).Exec(ctx)
		require.NoError(b, err)
	}

	b.Logf("Inserted %d recommendation score records, time: %v", count, time.Since(start))
}

func createCoveringIndex(b *testing.B, db *database.DB) {
	dropAllIndexes(b, db)

	ctx := b.Context()
	start := time.Now()
	_, err := db.Operator.Core.ExecContext(ctx, `
		CREATE INDEX idx_recom_covering
		ON recom_repo_scores (weight_name, repository_id, score DESC)`)
	require.NoError(b, err)
	b.Logf("Covering index creation time: %v", time.Since(start))
}

func createSpecializedIndex(b *testing.B, db *database.DB) {
	dropAllIndexes(b, db)

	ctx := b.Context()

	start := time.Now()
	_, err := db.Operator.Core.ExecContext(ctx, `
		CREATE INDEX idx_recom_total_weight_score
		ON recom_repo_scores (repository_id, score DESC)
		WHERE weight_name = 'total'`)
	require.NoError(b, err)
	b.Logf("Specialized index creation time: %v", time.Since(start))
}

func dropAllIndexes(b *testing.B, db *database.DB) {
	ctx := b.Context()
	start := time.Now()

	// Drop all custom indexes to ensure no index optimization
	_, _ = db.Operator.Core.ExecContext(ctx, "DROP INDEX IF EXISTS idx_recom_covering")
	_, _ = db.Operator.Core.ExecContext(ctx, "DROP INDEX IF EXISTS idx_recom_total_weight_score")

	b.Logf("All indexes dropped, time: %v", time.Since(start))
}

func benchmarkSortQuery(b *testing.B, db *database.DB) {
	b.ResetTimer()

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		var results []database.RecomRepoScore
		err := db.Operator.Core.NewSelect().
			Model(&results).
			Where("weight_name = ?", database.RecomWeightTotal).
			Order("score DESC").
			Limit(100).
			Scan(ctx)
		require.NoError(b, err)
	}

	b.ReportMetric(float64(b.Elapsed().Milliseconds()/int64(b.N)), "ms/sql_query")
}

func benchmarkCombinedQuery(b *testing.B, db *database.DB) {
	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		var results []struct {
			RepositoryID int64   `bun:"repository_id"`
			Popularity   float64 `bun:"popularity"`
		}

		query := `
			SELECT repos.*, COALESCE(r.score, 0) AS popularity
			FROM repositories repos
			LEFT JOIN recom_repo_scores r ON repos.id = r.repository_id
				AND r.weight_name = ?
			ORDER BY popularity DESC NULLS LAST
			LIMIT 50
		`

		err := db.Operator.Core.NewRaw(query, database.RecomWeightTotal, database.RecomWeightTotal).
			Scan(ctx, &results)
		require.NoError(b, err)
	}

	b.ReportMetric(float64(b.Elapsed().Milliseconds()/int64(b.N)), "ms/sql_query")
}

func benchmarkCombinedQuery_Where(b *testing.B, db *database.DB) {
	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		var results []struct {
			RepositoryID int64   `bun:"repository_id"`
			Popularity   float64 `bun:"popularity"`
		}

		query := `
			SELECT repos.*, COALESCE(r.score, 0) AS popularity
			FROM repositories repos
			LEFT JOIN recom_repo_scores r ON repos.id = r.repository_id
			WHERE r.weight_name = ?
			ORDER BY popularity DESC NULLS LAST
			LIMIT 50
		`

		err := db.Operator.Core.NewRaw(query, database.RecomWeightTotal, database.RecomWeightTotal).
			Scan(ctx, &results)
		require.NoError(b, err)
	}

	b.ReportMetric(float64(b.Elapsed().Milliseconds()/int64(b.N)), "ms/sql_query")
}
