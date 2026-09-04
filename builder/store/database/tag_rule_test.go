package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestTagRuleStore_FindByRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewTagRuleStoreWithDB(db)

	_, err := db.Core.NewInsert().Model(&database.TagRule{
		Category:  "foo",
		Namespace: "test",
		RepoName:  "bar",
		RepoType:  string(types.ModelRepo),
		TagName:   "t1",
	}).Exec(ctx)
	require.Nil(t, err)
	tr, err := store.FindByRepo(ctx, "foo", "test", "bar", string(types.ModelRepo))
	require.Nil(t, err)
	require.Equal(t, "t1", tr.TagName)

	_, err = store.FindByRepo(ctx, "foo", "test", "foo", string(types.ModelRepo))
	require.NotNil(t, err)
}

func TestTagRuleStore_SyncEvaluationDatasets(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewTagRuleStoreWithDB(db)

	amdEvalscopeTagCount, err := db.Core.NewSelect().
		Model((*database.Tag)(nil)).
		Where("name = ? AND category = ? AND scope IN (?)", "amd-evalscope", "runtime_framework", bun.In([]string{"model", "dataset"})).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, amdEvalscopeTagCount)

	_, err = db.Core.NewInsert().Model(&[]database.TagRule{
		{
			Namespace:        "old",
			RepoName:         "dataset",
			RepoType:         "dataset",
			Category:         "evaluation",
			TagName:          "other",
			RuntimeFramework: "evalscope",
			Source:           "ms",
		},
		{
			Namespace:        "evalscope",
			RepoName:         "aime25",
			RepoType:         "dataset",
			Category:         "evaluation",
			TagName:          "other",
			RuntimeFramework: "lm-evaluation-harness",
			Source:           "hf",
		},
		{
			Namespace:        "keep",
			RepoName:         "dataset",
			RepoType:         "dataset",
			Category:         "evaluation",
			TagName:          "other",
			RuntimeFramework: "amd-evalscope",
			Source:           "ms",
		},
	}).Exec(ctx)
	require.NoError(t, err)

	datasets := []types.EvaluationDatasetConfig{{
		Namespace:        "evalscope",
		RepoName:         "aime25",
		Category:         "evaluation",
		TagName:          "examination",
		RepoType:         "dataset",
		RuntimeFramework: "evalscope",
		Source:           "ms",
	}}
	require.NoError(t, store.SyncEvaluationDatasets(ctx, "evalscope", datasets, true))

	rules, err := store.FindAllByRepo(ctx, "evaluation", "evalscope", "aime25", "dataset")
	require.NoError(t, err)
	require.Len(t, rules, 2)
	rulesByFramework := make(map[string]database.TagRule, len(rules))
	for _, rule := range rules {
		rulesByFramework[rule.RuntimeFramework] = rule
	}
	require.Equal(t, "examination", rulesByFramework["evalscope"].TagName)
	require.Equal(t, "other", rulesByFramework["lm-evaluation-harness"].TagName)

	var staleCount int
	staleCount, err = db.Core.NewSelect().
		Model((*database.TagRule)(nil)).
		Where("namespace = ?", "old").
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, staleCount)

	require.NoError(t, store.SyncEvaluationDatasets(ctx, "amd-evalscope", datasets, false))
	var preservedCount int
	preservedCount, err = db.Core.NewSelect().
		Model((*database.TagRule)(nil)).
		Where("namespace = ? AND runtime_framework = ?", "keep", "amd-evalscope").
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, preservedCount)

	require.Error(t, store.SyncEvaluationDatasets(ctx, "evalscope", nil, true))
	var evalscopeCount int
	evalscopeCount, err = db.Core.NewSelect().
		Model((*database.TagRule)(nil)).
		Where("runtime_framework = ?", "evalscope").
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, evalscopeCount)
}
