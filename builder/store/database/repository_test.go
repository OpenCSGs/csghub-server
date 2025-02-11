package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	_, err := store.CreateRepo(ctx, database.Repository{
		Name:    "repo1",
		UserID:  123,
		GitPath: "foos_u/bar",
	})
	require.Nil(t, err)

	rp := &database.Repository{}
	err = db.Core.NewSelect().Model(rp).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rp, err = store.FindById(ctx, rp.ID)
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rps, err := store.FindByIds(ctx, []int64{rp.ID})
	require.Nil(t, err)
	require.Equal(t, 1, len(rps))
	require.Equal(t, "repo1", rps[0].Name)

	rps, err = store.All(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(rps))
	require.Equal(t, "repo1", rps[0].Name)

	rp, err = store.Find(ctx, "u", "foO", "bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	exist, err := store.Exists(ctx, "foO", "u", "bar")
	require.Nil(t, err)
	require.True(t, exist)

	rp, err = store.FindByPath(ctx, "foO", "u", "bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rp, err = store.FindByGitPath(ctx, "foos_u/bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rps, err = store.FindByGitPaths(ctx, []string{"foos_u/bAr"})
	require.Nil(t, err)
	require.Equal(t, 1, len(rps))
	require.Equal(t, "repo1", rps[0].Name)

	rpsp, err := store.ByUser(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, 1, len(rpsp))
	require.Equal(t, "repo1", rpsp[0].Name)
	rpsp, err = store.ByUser(ctx, 125)
	require.Nil(t, err)
	require.Equal(t, 0, len(rpsp))

	rpn := *rp
	rpn.Name = "repo1-new"
	_, err = store.UpdateRepo(ctx, rpn)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(rp).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "repo1-new", rp.Name)

	err = store.DeleteRepo(ctx, database.Repository{
		ID: rp.ID,
	})
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(rp).Where("user_id=?", 123).Scan(ctx)
	require.NotNil(t, err)

	_, err = store.UpdateOrCreateRepo(ctx, database.Repository{
		Name:           "repo3",
		UserID:         231,
		Path:           "bars_u/bar",
		RepositoryType: types.CodeRepo,
	})
	require.Nil(t, err)
	rp = &database.Repository{}
	err = db.Core.NewSelect().Model(rp).Where("user_id=?", 231).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "repo3", rp.Name)

	_, err = store.UpdateOrCreateRepo(ctx, database.Repository{
		Name:           "repo3n",
		UserID:         231,
		Path:           "bars_u/bar",
		RepositoryType: types.CodeRepo,
	})
	require.Nil(t, err)
	rp = &database.Repository{}
	err = db.Core.NewSelect().Model(rp).Where("user_id=?", 231).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "repo3n", rp.Name)

	cnt, err := store.CountByRepoType(ctx, types.CodeRepo)
	require.Nil(t, err)
	require.Equal(t, 1, cnt)

}

func TestRepoStore_UpdateRepoFileDownloads(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	repo, err := store.CreateRepo(ctx, database.Repository{
		Name:    "repo1",
		UserID:  123,
		GitPath: "foos_u/bar",
	})
	require.Nil(t, err)

	dt := time.Date(2022, 12, 11, 0, 0, 0, 0, time.UTC)
	dw := &database.RepositoryDownload{}
	// create
	err = store.UpdateRepoFileDownloads(ctx, repo, dt, 111)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dw).Where("repository_id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 111, int(dw.ClickDownloadCount))
	err = db.Core.NewSelect().Model(repo).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 111, int(repo.DownloadCount))

	// update
	err = store.UpdateRepoFileDownloads(ctx, repo, dt, 5)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dw).Where("repository_id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 116, int(dw.ClickDownloadCount))
	err = db.Core.NewSelect().Model(repo).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 116, int(repo.DownloadCount))

}

func TestRepoStore_UpdateRepoCloneDownloads(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	repo, err := store.CreateRepo(ctx, database.Repository{
		Name:    "repo1",
		UserID:  123,
		GitPath: "foos_u/bar",
	})
	require.Nil(t, err)

	dt := time.Date(2022, 12, 11, 0, 0, 0, 0, time.UTC)
	dw := &database.RepositoryDownload{}
	// create
	err = store.UpdateRepoCloneDownloads(ctx, repo, dt, 111)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dw).Where("repository_id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 111, int(dw.CloneCount))
	err = db.Core.NewSelect().Model(repo).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 111, int(repo.DownloadCount))

	// update
	err = store.UpdateRepoCloneDownloads(ctx, repo, dt, 5)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(dw).Where("repository_id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	// clone count will be override, not add to previous
	require.Equal(t, 5, int(dw.CloneCount))
	err = db.Core.NewSelect().Model(repo).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 5, int(repo.DownloadCount))

}

func TestRepoStore_Tags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	repo, err := store.CreateRepo(ctx, database.Repository{
		Name:   "repo1",
		UserID: 123,
	})
	require.Nil(t, err)

	tag := &database.Tag{
		Name:     "tg",
		Category: "foo",
	}
	_, err = db.Core.NewInsert().Model(tag).Exec(ctx, tag)
	require.Nil(t, err)
	tag2 := &database.Tag{
		Name:     "tg2",
		Category: "bar",
	}
	_, err = db.Core.NewInsert().Model(tag2).Exec(ctx, tag2)
	require.Nil(t, err)

	rtags := []database.RepositoryTag{
		{TagID: tag.ID, RepositoryID: repo.ID, Count: 1},
		{TagID: tag2.ID, RepositoryID: repo.ID, Count: 1},
	}
	err = store.BatchCreateRepoTags(ctx, rtags)
	require.Nil(t, err)

	tags, err := store.Tags(ctx, repo.ID)
	require.Nil(t, err)
	require.Equal(t, 2, len(tags))
	require.Equal(t, tags[0].Name, "tg")
	require.Equal(t, tags[1].Name, "tg2")

	tags, err = store.TagsWithCategory(ctx, repo.ID, "foo")
	require.Nil(t, err)
	require.Equal(t, 1, len(tags))
	require.Equal(t, tags[0].Name, "tg")

	ids, err := store.TagIDs(ctx, repo.ID, "foo")
	require.Nil(t, err)
	require.Equal(t, []int64{tag.ID}, ids)
}

func TestRepoStore_SetUpdateTimeByPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	repo, err := store.CreateRepo(ctx, database.Repository{
		Name:    "repo1",
		UserID:  123,
		GitPath: "foos_u/bar",
	})
	require.Nil(t, err)

	dt := time.Date(2022, 12, 6, 1, 2, 0, 0, time.UTC)
	err = store.SetUpdateTimeByPath(ctx, "foo", "u", "bar", dt)
	require.Nil(t, err)

	err = db.Core.NewSelect().Model(repo).WherePK().Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, dt, repo.UpdatedAt)

}

func TestRepoStore_PublicToUserSimple(t *testing.T) {
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
	repos, _, err := rs.PublicToUser(ctx, repo.RepositoryType, []int64{1}, filter, 20, 1, false)
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
	repos, _, err = rs.PublicToUser(ctx, repo.RepositoryType, []int64{1}, filter, 20, 1, false)
	require.Nil(t, err)
	require.NotNil(t, repos)
}

func TestRepoStore_PublicToUser(t *testing.T) {

	cases := []struct {
		admin    bool
		repoType types.RepositoryType
		source   string
		search   string
		tags     []types.TagReq
		sort     string
		expected []string
	}{
		{
			admin: false, repoType: types.CodeRepo,
			expected: []string{"rp1", "rp2", "rp4", "rp5", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo, source: string(types.HuggingfaceSource),
			expected: []string{"rp2"},
		},
		{
			admin: true, repoType: types.CodeRepo,
			expected: []string{"rp1", "rp2", "rp3", "rp4", "rp5", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo, search: "rp4",
			expected: []string{"rp4", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo, sort: "most_download",
			expected: []string{"rp1", "rp2", "rp4", "rp5", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo, sort: "trending",
			expected: []string{"rp1", "rp2", "rp4", "rp5", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo, tags: []types.TagReq{{Name: "foo"}},
			expected: []string{"rp4"},
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			store := database.NewRepoStoreWithDB(db)

			repos := []*database.Repository{
				{
					Name: "rp1", Path: "rp1", UserID: 123, RepositoryType: types.CodeRepo,
					DownloadCount: 10,
				},
				{
					Name: "rp2", Path: "rp2", UserID: 123, RepositoryType: types.CodeRepo,
					Private: true, Source: types.HuggingfaceSource,
					DownloadCount: 10,
				},
				{
					Name: "rp3", Path: "rp3", UserID: 456, RepositoryType: types.CodeRepo,
					Private:       true,
					DownloadCount: 15,
				},
				{
					Name: "rp4", Path: "rp4", UserID: 456, RepositoryType: types.CodeRepo,
					Private: false, Tags: []database.Tag{{Name: "foo"}},
					DownloadCount: 10,
				},
				{
					Name: "rp5", Path: "rp5", UserID: 789, RepositoryType: types.CodeRepo,
					Private: false, Tags: []database.Tag{{Name: "bar"}},
					DownloadCount: 10,
				},
				{
					Name: "rp6", Path: "rp6", UserID: 789, RepositoryType: types.CodeRepo,
					Private: false, Description: "rp4desc",
					DownloadCount: 10,
				},
			}

			for _, repo := range repos {
				repo.GitPath = repo.Path
				rn, err := store.CreateRepo(ctx, *repo)
				require.Nil(t, err)
				for _, tag := range repo.Tags {
					_, err = db.Core.NewInsert().Model(&tag).Exec(ctx, &tag)
					require.Nil(t, err)
					rtags := []database.RepositoryTag{
						{TagID: tag.ID, RepositoryID: rn.ID, Count: 1},
					}
					err = store.BatchCreateRepoTags(ctx, rtags)
					require.Nil(t, err)
				}
			}

			rs, count, err := store.PublicToUser(ctx, c.repoType, []int64{123}, &types.RepoFilter{
				Tags:   c.tags,
				Sort:   c.sort,
				Search: c.search,
				Source: c.source,
			}, 10, 1, c.admin)
			require.Nil(t, err)
			names := []string{}
			for _, r := range rs {
				names = append(names, r.Name)
			}
			require.Equal(t, len(c.expected), count)
			require.Equal(t, c.expected, names)

		})
	}
}

func TestRepoStore_IsMirrorRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	rn, err := store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns/n",
	})
	require.Nil(t, err)
	mi := &database.Mirror{RepositoryID: rn.ID}
	_, err = db.Core.NewInsert().Model(mi).Exec(ctx)
	require.Nil(t, err)
	m, err := store.IsMirrorRepo(ctx, types.CodeRepo, "ns", "n")
	require.Nil(t, err)
	require.True(t, m)
	m, err = store.IsMirrorRepo(ctx, types.CodeRepo, "ns", "n2")
	require.NotNil(t, err)
	require.False(t, m)

}

func TestRepoStore_WithMirror(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	rn, err := store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns/n",
	})
	require.Nil(t, err)
	mi := &database.Mirror{RepositoryID: rn.ID}
	_, err = db.Core.NewInsert().Model(mi).Exec(ctx)
	require.Nil(t, err)
	_, err = store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns2/n",
		Path:    "zzz",
	})
	require.Nil(t, err)
	rs, count, err := store.WithMirror(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "codes_ns/n", rs[0].GitPath)

}

func TestRepoStore_ListRepoPublicToUserByRepoIDs(t *testing.T) {

	cases := []struct {
		admin    bool
		repoType types.RepositoryType
		search   string
		sort     string
		expected []string
	}{
		{
			admin: false, repoType: types.CodeRepo,
			expected: []string{"rp1", "rp2", "rp4", "rp5", "rp6"},
		},
		{
			admin: true, repoType: types.CodeRepo,
			expected: []string{"rp1", "rp2", "rp4", "rp5", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo, search: "rp4",
			expected: []string{"rp4", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo, sort: "most_download",
			expected: []string{"rp1", "rp2", "rp4", "rp5", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo, sort: "trending",
			expected: []string{"rp1", "rp2", "rp4", "rp5", "rp6"},
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			store := database.NewRepoStoreWithDB(db)

			repos := []*database.Repository{
				{
					Name: "rp1", Path: "rp1", UserID: 123, RepositoryType: types.CodeRepo,
					DownloadCount: 10,
				},
				{
					Name: "rp2", Path: "rp2", UserID: 123, RepositoryType: types.CodeRepo,
					Private:       true,
					DownloadCount: 10,
				},
				{
					Name: "rp3", Path: "rp3", UserID: 456, RepositoryType: types.CodeRepo,
					Private:       true,
					DownloadCount: 15,
				},
				{
					Name: "rp4", Path: "rp4", UserID: 456, RepositoryType: types.CodeRepo,
					Private: false, Tags: []database.Tag{{Name: "foo"}},
					DownloadCount: 10,
				},
				{
					Name: "rp5", Path: "rp5", UserID: 789, RepositoryType: types.CodeRepo,
					Private: false, Tags: []database.Tag{{Name: "bar"}},
					DownloadCount: 10,
				},
				{
					Name: "rp6", Path: "rp6", UserID: 789, RepositoryType: types.CodeRepo,
					Private: false, Description: "rp4desc",
					DownloadCount: 10,
				},
			}

			rids := []int64{}
			for _, repo := range repos {
				repo.GitPath = repo.Path
				rn, err := store.CreateRepo(ctx, *repo)
				require.Nil(t, err)
				rids = append(rids, rn.ID)
				for _, tag := range repo.Tags {
					_, err = db.Core.NewInsert().Model(&tag).Exec(ctx, &tag)
					require.Nil(t, err)
					rtags := []database.RepositoryTag{
						{TagID: tag.ID, RepositoryID: rn.ID, Count: 1},
					}
					err = store.BatchCreateRepoTags(ctx, rtags)
					require.Nil(t, err)
				}
			}

			rs, count, err := store.ListRepoPublicToUserByRepoIDs(ctx, c.repoType, 123, c.search, c.sort, 10, 1, rids)
			require.Nil(t, err)
			names := []string{}
			for _, r := range rs {
				names = append(names, r.Name)
			}
			require.Equal(t, len(c.expected), count)
			require.Equal(t, c.expected, names)

		})
	}
}

func TestRepoStore_CleanRelationsByRepoID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	rn, err := store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns/n",
	})
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RepositoriesRuntimeFramework{
		RepoID: rn.ID,
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.UserLike{
		RepoID: rn.ID,
	}).Exec(ctx)
	require.Nil(t, err)
	count, err := db.Core.NewSelect().Model(&database.RepositoriesRuntimeFramework{}).Where("repo_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	count, err = db.Core.NewSelect().Model(&database.UserLike{}).Where("repo_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	err = store.CleanRelationsByRepoID(ctx, rn.ID)
	require.Nil(t, err)

	count, err = db.Core.NewSelect().Model(&database.RepositoriesRuntimeFramework{}).Where("repo_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)
	count, err = db.Core.NewSelect().Model(&database.UserLike{}).Where("repo_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)

}

func TestRepoStore_DeleteAllFiles(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	_, err := db.Core.NewInsert().Model(&database.File{RepositoryID: 123}).Exec(ctx)
	require.Nil(t, err)
	err = store.DeleteAllFiles(ctx, 123)
	require.Nil(t, err)
	count, err := db.Core.NewSelect().Model(&database.File{}).Where("repository_id = ?", 123).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)
}

func TestRepoStore_DeleteAllTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	_, err := db.Core.NewInsert().Model(&database.RepositoryTag{RepositoryID: 123}).Exec(ctx)
	require.Nil(t, err)
	err = store.DeleteAllTags(ctx, 123)
	require.Nil(t, err)
	count, err := db.Core.NewSelect().Model(&database.RepositoryTag{}).Where("repository_id = ?", 123).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)
}

func TestRepoStore_UpdateLicenseByTag(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	rn, err := store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns/n",
		License: "foo",
	})
	require.Nil(t, err)
	tag := &database.Tag{
		Category: "license",
		Name:     "MIT",
	}
	_, err = db.Core.NewInsert().Model(tag).Exec(ctx, tag)
	require.Nil(t, err)
	rtags := []database.RepositoryTag{
		{TagID: tag.ID, RepositoryID: rn.ID, Count: 1},
	}
	err = store.BatchCreateRepoTags(ctx, rtags)
	require.Nil(t, err)

	err = store.UpdateLicenseByTag(ctx, rn.ID)
	require.Nil(t, err)

	rn, err = store.FindById(ctx, rn.ID)
	require.Nil(t, err)
	require.Equal(t, "MIT", rn.License)
}

func TestRepoStore_GetRepoRuntimeByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	repo, err := store.CreateRepo(ctx, database.Repository{
		Path:           "codes_ns/n",
		License:        "foo",
		RepositoryType: types.ModelRepo,
	})
	require.Nil(t, err)

	rf := &database.RepositoriesRuntimeFramework{
		RepoID:             repo.ID,
		RuntimeFrameworkID: 999,
	}
	_, err = db.Core.NewInsert().Model(rf).Exec(ctx, rf)
	require.Nil(t, err)

	rs, err := store.GetRepoWithRuntimeByID(ctx, rf.RuntimeFrameworkID, []string{"codes_ns/n"})
	require.Nil(t, err)
	require.Equal(t, 1, len(rs))

	rs, err = store.GetRepoWithoutRuntimeByID(ctx, rf.RuntimeFrameworkID, []string{"codes_ns/n"}, 1000, 0)
	require.Nil(t, err)
	require.Equal(t, 0, len(rs))

}

func TestRepoStore_BatchMethods(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	repos := []*database.Repository{
		{
			Name: "rp1", Path: "rp1", UserID: 123, RepositoryType: types.CodeRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
			Source:               types.HuggingfaceSource,
		},
		{
			Name: "rp2", Path: "rp2", UserID: 123, RepositoryType: types.CodeRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
			Source:               types.HuggingfaceSource,
		},
		{
			Name: "rp3", Path: "rp3", UserID: 123, RepositoryType: types.CodeRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
			Source:               types.HuggingfaceSource,
		},
		{
			Name: "rp4", Path: "rp4", UserID: 456, RepositoryType: types.CodeRepo,
			SensitiveCheckStatus: types.SensitiveCheckPass,
			Source:               types.HuggingfaceSource,
		},
		{
			Name: "rp5", Path: "rp5", UserID: 456, RepositoryType: types.DatasetRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		},
	}

	names := func(rs []database.Repository) []string {
		names := []string{}
		for _, r := range rs {
			names = append(names, r.Name)
		}
		return names
	}

	rids := []int64{}
	for _, repo := range repos {
		repo.GitPath = repo.Path
		rn, err := store.CreateRepo(ctx, *repo)
		require.Nil(t, err)
		rids = append(rids, rn.ID)
	}
	rs, err := store.BatchGet(ctx, types.CodeRepo, 0, 10)
	require.Nil(t, err)
	require.Equal(t, len(rs), 3)
	require.Equal(t, []string{"rp1", "rp2", "rp3"}, names(rs))

	rs, err = store.BatchGet(ctx, types.CodeRepo, rids[1], 10)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.Equal(t, []string{"rp3"}, names(rs))

	rs, err = store.BatchGet(ctx, types.CodeRepo, 0, 1)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.Equal(t, []string{"rp1"}, names(rs))

	rs, err = store.FindWithBatch(ctx, 2, 1)
	require.Nil(t, err)
	require.Equal(t, len(rs), 2)
	require.Equal(t, []string{"rp3", "rp2"}, names(rs))

	rs, err = store.FindWithBatch(ctx, 2, 0, types.DatasetRepo)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.Equal(t, []string{"rp5"}, names(rs))

	rs, err = store.ByUser(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, len(rs), 3)
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3"}, names(rs))

	rs, err = store.FindByRepoSourceWithBatch(ctx, types.HuggingfaceSource, 2, 1)
	require.Nil(t, err)
	require.Equal(t, len(rs), 2)
	require.Equal(t, []string{"rp2", "rp1"}, names(rs))
}
