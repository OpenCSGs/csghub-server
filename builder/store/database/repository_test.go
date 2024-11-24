package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoStore_PublicToUser(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// insert a new repo
	rs := database.NewRepoStoreWithDB(db)
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("datasets_%s/%s", userName, repoName),
		Name:           repoName,
		DefaultBranch:  "main",
		Nickname:       "ww",
		Description:    "ww",
		Private:        false,
		RepositoryType: types.DatasetRepo,
	})
	require.Empty(t, err)
	require.NotNil(t, repo)

	ts := database.NewTagStoreWithDB(db)
	var tags []*database.Tag
	tags = append(tags, &database.Tag{
		Name:     "tag_" + uuid.NewString(),
		Category: "evaluation",
		Group:    "",
		Scope:    database.DatasetTagScope,
		BuiltIn:  true,
		ShowName: "",
	})
	tags = append(tags, &database.Tag{
		Name:     "tag_" + uuid.NewString(),
		Category: "runtime_framework",
		Group:    "",
		Scope:    database.DatasetTagScope,
		BuiltIn:  true,
		ShowName: "",
	})
	t1, err := ts.FindOrCreate(ctx, *tags[0])
	require.Nil(t, err)
	t2, err := ts.FindOrCreate(ctx, *tags[1])
	require.Nil(t, err)
	err = ts.UpsertRepoTags(ctx, repo.ID, []int64{}, []int64{t1.ID, t2.ID})
	require.Nil(t, err)
	var tagsfilter []types.TagReq
	tagsfilter = append(tagsfilter, types.TagReq{
		Name:     tags[0].Name,
		Category: tags[0].Category,
	})
	filter := &types.RepoFilter{
		Tags: tagsfilter,
		Sort: "recently_update",
	}
	// case 1: one tag
	repos, _, err := rs.PublicToUser(ctx, repo.RepositoryType, []int64{1}, filter, 20, 1)
	require.Nil(t, err)
	require.NotNil(t, repos)

	tagsfilter = append(tagsfilter, types.TagReq{
		Name:     tags[1].Name,
		Category: tags[1].Category,
	})
	filter = &types.RepoFilter{
		Tags: tagsfilter,
		Sort: "recently_update",
	}
	// case 2: two tag
	repos, _, err = rs.PublicToUser(ctx, repo.RepositoryType, []int64{1}, filter, 20, 1)
	require.Nil(t, err)
	require.NotNil(t, repos)
}
