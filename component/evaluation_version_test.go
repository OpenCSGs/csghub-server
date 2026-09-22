package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

const (
	wukongMainCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	wukongOldCommit  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// expectModel makes the model lookup succeed with the given default branch.
func expectModel(c *testEvaluationWithMocks, ctx context.Context, namespace, name, defaultBranch string) {
	c.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, namespace, name).Return(&database.Model{
		ID:         1,
		Repository: &database.Repository{DefaultBranch: defaultBranch},
	}, nil).Maybe()
}

// expectCommit resolves one model revision to a commit id.
func expectCommit(c *testEvaluationWithMocks, ctx context.Context, namespace, name, ref, commitID string) {
	c.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: namespace, Name: name, Ref: ref, RepoType: types.ModelRepo,
	}).Return(&types.Commit{ID: commitID}, nil).Maybe()
}

func TestEvaluationComponent_resolveModelVersions(t *testing.T) {
	ctx := context.TODO()

	t.Run("same model at several commits", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		expectModel(c, ctx, "opencsg", "wukong", "main")
		expectCommit(c, ctx, "opencsg", "wukong", wukongMainCommit, wukongMainCommit)
		expectCommit(c, ctx, "opencsg", "wukong", wukongOldCommit, wukongOldCommit)

		req := types.EvaluationReq{Models: []types.EvaluationModelRef{
			{RepoId: "opencsg/wukong", Revision: wukongMainCommit},
			{RepoId: "opencsg/wukong", Revision: wukongOldCommit},
		}}
		require.NoError(t, c.resolveModelVersions(ctx, &req))
		require.Equal(t, []string{"opencsg/wukong", "opencsg/wukong"}, req.ModelIds)
		require.Equal(t, []string{wukongMainCommit, wukongOldCommit}, req.Revisions)
	})

	t.Run("empty revision pins the default branch head", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		expectModel(c, ctx, "opencsg", "wukong", "main")
		expectCommit(c, ctx, "opencsg", "wukong", "main", wukongMainCommit)

		req := types.EvaluationReq{Models: []types.EvaluationModelRef{{RepoId: "opencsg/wukong"}}}
		require.NoError(t, c.resolveModelVersions(ctx, &req))
		require.Equal(t, []string{wukongMainCommit}, req.Revisions)
	})

	t.Run("legacy model_id and model_ids still resolve", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		expectModel(c, ctx, "opencsg", "wukong", "main")
		expectModel(c, ctx, "opencsg", "starcoder", "main")
		expectCommit(c, ctx, "opencsg", "wukong", "main", wukongMainCommit)
		expectCommit(c, ctx, "opencsg", "starcoder", "main", wukongOldCommit)

		req := types.EvaluationReq{
			ModelIds: []string{"opencsg/wukong"},
			ModelId:  "opencsg/starcoder",
		}
		require.NoError(t, c.resolveModelVersions(ctx, &req))
		require.Equal(t, []string{"opencsg/wukong", "opencsg/starcoder"}, req.ModelIds)
		require.Equal(t, []string{wukongMainCommit, wukongOldCommit}, req.Revisions)
	})

	t.Run("models takes precedence over model_ids", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		expectModel(c, ctx, "opencsg", "wukong", "main")
		expectCommit(c, ctx, "opencsg", "wukong", wukongOldCommit, wukongOldCommit)

		req := types.EvaluationReq{
			ModelIds: []string{"opencsg/ignored"},
			Models:   []types.EvaluationModelRef{{RepoId: "opencsg/wukong", Revision: wukongOldCommit}},
		}
		require.NoError(t, c.resolveModelVersions(ctx, &req))
		require.Equal(t, []string{"opencsg/wukong"}, req.ModelIds)
	})

	t.Run("duplicate model and revision is collapsed", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		expectModel(c, ctx, "opencsg", "wukong", "main")
		expectCommit(c, ctx, "opencsg", "wukong", "main", wukongMainCommit)
		expectCommit(c, ctx, "opencsg", "wukong", wukongMainCommit, wukongMainCommit)

		req := types.EvaluationReq{Models: []types.EvaluationModelRef{
			{RepoId: "opencsg/wukong"},
			{RepoId: "opencsg/wukong", Revision: wukongMainCommit},
		}}
		require.NoError(t, c.resolveModelVersions(ctx, &req))
		require.Equal(t, []string{"opencsg/wukong"}, req.ModelIds)
		require.Equal(t, []string{wukongMainCommit}, req.Revisions)
	})

	t.Run("unknown revision is rejected", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		expectModel(c, ctx, "opencsg", "wukong", "main")
		// Gitaly reports an unknown revision as an empty commit rather than an error.
		c.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
			Namespace: "opencsg", Name: "wukong", Ref: "nope", RepoType: types.ModelRepo,
		}).Return(&types.Commit{}, nil).Once()

		req := types.EvaluationReq{Models: []types.EvaluationModelRef{
			{RepoId: "opencsg/wukong", Revision: "nope"},
		}}
		err := c.resolveModelVersions(ctx, &req)
		require.ErrorContains(t, err, "revision nope does not exist in opencsg/wukong")
	})

	t.Run("git failure is propagated", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		expectModel(c, ctx, "opencsg", "wukong", "main")
		c.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
			Namespace: "opencsg", Name: "wukong", Ref: "main", RepoType: types.ModelRepo,
		}).Return(nil, errors.New("gitaly down")).Once()

		req := types.EvaluationReq{ModelIds: []string{"opencsg/wukong"}}
		err := c.resolveModelVersions(ctx, &req)
		require.ErrorContains(t, err, "gitaly down")
	})

	t.Run("no model is rejected", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		req := types.EvaluationReq{}
		require.ErrorContains(t, c.resolveModelVersions(ctx, &req), "at least one model is required")
	})

	t.Run("too many model versions is rejected", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		var refs []types.EvaluationModelRef
		for i := 0; i <= maxEvaluationModelVersions; i++ {
			refs = append(refs, types.EvaluationModelRef{RepoId: "opencsg/wukong"})
		}
		req := types.EvaluationReq{Models: refs}
		require.ErrorContains(t, c.resolveModelVersions(ctx, &req), "at most 10 model versions")
	})

	t.Run("invalid model id is rejected", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		req := types.EvaluationReq{ModelIds: []string{"badmodelid"}}
		require.ErrorContains(t, c.resolveModelVersions(ctx, &req), "invalid model id format: badmodelid")
	})
}

func TestEvaluationComponent_resolveDatasetVersions(t *testing.T) {
	ctx := context.TODO()

	t.Run("dataset commit is pinned alongside the branch", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		c.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "opencsg", "hellaswag").Return(&database.Repository{
			ID:            1,
			Path:          "opencsg/hellaswag",
			DefaultBranch: "main",
			HFPath:        "Rowan/hellaswag",
		}, nil).Once()
		c.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
			Namespace: "opencsg", Name: "hellaswag", Ref: "main", RepoType: types.DatasetRepo,
		}).Return(&types.Commit{ID: wukongMainCommit}, nil).Once()

		got, err := c.GenerateMirrorRepoIds(ctx, []string{"opencsg/hellaswag"})
		require.NoError(t, err)
		require.Equal(t, []string{"Rowan/hellaswag"}, got.Paths)
		require.Equal(t, []string{"main"}, got.Revisions)
		require.Equal(t, []string{wukongMainCommit}, got.Commits)
	})

	t.Run("unresolvable dataset commit does not fail the request", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		c.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "opencsg", "hellaswag").Return(&database.Repository{
			ID:            1,
			Path:          "opencsg/hellaswag",
			DefaultBranch: "main",
		}, nil).Once()
		c.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
			Namespace: "opencsg", Name: "hellaswag", Ref: "main", RepoType: types.DatasetRepo,
		}).Return(nil, errors.New("gitaly down")).Once()

		got, err := c.generateDatasetsAndTasks(ctx, []string{"opencsg/hellaswag"})
		require.NoError(t, err)
		require.Equal(t, []string{"opencsg/hellaswag"}, got.Paths)
		require.Equal(t, []string{"main"}, got.Revisions)
		require.Equal(t, []string{""}, got.Commits)
	})
}

func TestNormalizeFrameworkConfig(t *testing.T) {
	const defaultGen = `"generation_config":{"do_sample":false,"max_tokens":30000}`
	cases := []struct {
		name      string
		frameName string
		raw       string
		want      string
		wantErr   string
	}{
		{name: "an unsupported framework keeps an empty configuration",
			frameName: "opencompass", raw: "", want: ""},
		{name: "an unsupported framework rejects a configuration",
			frameName: "opencompass", raw: `{"limit":10}`, wantErr: "does not support framework_config"},
		{name: "an omitted configuration is filled with what will run",
			frameName: "evalscope", raw: "", want: "{" + defaultGen + `,"limit":10}`},
		{name: "a blank configuration is filled with what will run",
			frameName: "evalscope", raw: "   ", want: "{" + defaultGen + `,"limit":10}`},
		{name: "an omitted limit is filled in",
			frameName: "evalscope", raw: `{"generation_config":{"max_tokens":128}}`,
			want: `{"generation_config":{"max_tokens":128},"limit":10}`},
		{name: "an empty generation config is filled in",
			frameName: "evalscope", raw: `{"generation_config":{},"limit":5}`,
			want: "{" + defaultGen + `,"limit":5}`},
		{name: "explicit values are preserved",
			frameName: "evalscope", raw: `{"generation_config":{"max_tokens":128},"limit":200}`,
			want: `{"generation_config":{"max_tokens":128},"limit":200}`},
		{name: "an explicit zero limit is rejected",
			frameName: "evalscope", raw: `{"limit":0}`, wantErr: "limit must be at least 1"},
		{name: "a negative limit is rejected",
			frameName: "evalscope", raw: `{"limit":-1}`, wantErr: "limit must be at least 1"},
		{name: "a field outside the contract is rejected",
			frameName: "evalscope", raw: `{"num_fewshot":5}`, wantErr: "invalid framework_config"},
		{name: "malformed json is rejected",
			frameName: "evalscope", raw: `{"limit":`, wantErr: "invalid framework_config"},
		{name: "amd evalscope is treated the same",
			frameName: "amd-evalscope", raw: `{"limit":5}`, want: "{" + defaultGen + `,"limit":5}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := normalizeFrameworkConfig(c.frameName, c.raw)
			if c.wantErr != "" {
				require.ErrorContains(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}
}
