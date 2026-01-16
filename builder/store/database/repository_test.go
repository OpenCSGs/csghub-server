package database_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)

	_, err := store.CreateRepo(ctx, database.Repository{
		Name:           "repo1",
		UserID:         123,
		GitPath:        "models_u/bar",
		Path:           "u/bar",
		RepositoryType: types.ModelRepo,
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

	rp, err = store.Find(ctx, "u", "model", "bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	exist, err := store.Exists(ctx, "model", "u", "bar")
	require.Nil(t, err)
	require.True(t, exist)

	rp, err = store.FindByPath(ctx, "model", "u", "bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rp, err = store.FindByGitPath(ctx, "models_u/bAr")
	require.Nil(t, err)
	require.Equal(t, "repo1", rp.Name)

	rps, err = store.FindByGitPaths(ctx, []string{"models_u/bAr"})
	require.Nil(t, err)
	require.Equal(t, 1, len(rps))
	require.Equal(t, "repo1", rps[0].Name)

	rpsp, err := store.ByUser(ctx, 123, 100, 0)
	require.Nil(t, err)
	require.Equal(t, 1, len(rpsp))
	require.Equal(t, "repo1", rpsp[0].Name)
	rpsp, err = store.ByUser(ctx, 125, 100, 0)
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
		Name:           "repo1",
		UserID:         123,
		GitPath:        "models_u/bar",
		RepositoryType: types.ModelRepo,
		Path:           "u/bar",
	})
	require.Nil(t, err)

	dt := time.Date(2022, 12, 6, 1, 2, 0, 0, time.UTC)
	err = store.SetUpdateTimeByPath(ctx, types.ModelRepo, "u", "bar", dt)
	require.Nil(t, err)

	err = db.Core.NewSelect().Model(repo).WherePK().Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, dt, repo.UpdatedAt)

}

func TestRepoStore_PublicToUserSimple(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	redisClient := tests.InitTestRedis()
	defer redisClient.Close()
	config := &config.Config{}
	config.Search.RepoSearchCacheTTL = 300
	config.Search.RepoSearchLimit = 10
	config.Database.Driver = "pg"
	config.Database.SearchConfiguration = "opencsgchinese"
	rs := database.NewRepoStoreWithCache(config, db, cache.NewCacheWithClient(ctx, redisClient))
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
	ds := database.NewDatasetStoreWithDB(db)
	_, err = ds.Create(ctx, database.Dataset{
		RepositoryID: repo.ID,
	})
	require.Empty(t, err)

	ts := database.NewTagStoreWithDB(db)
	var tags []*database.Tag
	tags = append(tags, &database.Tag{
		Name:     "tag_" + uuid.NewString(),
		Category: "evaluation",
		Group:    "",
		Scope:    types.DatasetTagScope,
		BuiltIn:  true,
		ShowName: "",
	})
	tags = append(tags, &database.Tag{
		Name:     "tag_" + uuid.NewString(),
		Category: "runtime_framework",
		Group:    "",
		Scope:    types.DatasetTagScope,
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
	repos, _, err := rs.PublicToUser(ctx, repo.RepositoryType, []int64{1}, filter, 20, 1, true)
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
	repos, _, err = rs.PublicToUser(ctx, repo.RepositoryType, []int64{1}, filter, 20, 1, true)
	require.Nil(t, err)
	require.NotNil(t, repos)
}

func defineTestCases(hasZhparser bool) []repoSearchCase {
	commonCases := []repoSearchCase{
		{
			admin: false, repoType: types.CodeRepo,
			search:   "billionaire",
			expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
		},
		// test cache
		{
			admin: false, repoType: types.CodeRepo,
			search:   "billionaire",
			expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "stable diff",
			expected: []string{"Stable"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "Qwen2.5-Coder-32B-Instruct",
			expected: []string{"Qwen2.5"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "mason/chatglm3-6b-32k",
			expected: []string{"ChatGlm3"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "Qwen/Qwen2-1.5B",
			expected: []string{"Qwen2-1.5B", "Qwen2-1.5B-Instruct-GGUF"},
		},
	}

	if !hasZhparser {
		return commonCases
	}

	zhparserCases := []repoSearchCase{
		{
			admin: false, repoType: types.CodeRepo,
			search:   "中医经典",
			expected: []string{"ChineseMedicalBooksCollection"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "中医 经典",
			expected: []string{"ChineseMedicalBooksCollection"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			search:   "生物 医学",
			expected: []string{"ChineseBlue"},
		},
	}

	return append(zhparserCases, commonCases...)
}

func setupTestRepositories(ctx context.Context, store database.RepoStore, db *database.DB) ([]*database.Repository, error) {
	repos := []*database.Repository{
		{
			Name:           "ChineseMedicalBooksCollection",
			Path:           "billionaire/ChineseMedicalBooksCollection",
			Nickname:       "历代中医文献数据库",
			Description:    "历代中医文献数据库,内含约700本中医经典书籍。全部txt文件",
			UserID:         123,
			RepositoryType: types.CodeRepo,
			DownloadCount:  10,
			Private:        false,
			Tags:           []database.Tag{{Name: "foo"}},
		},
		{
			Name:           "ChineseBlue",
			Path:           "billionaire/ChineseBlue",
			Nickname:       "ChineseBlue中文生物医学文本",
			Description:    "ChinesseBLUE基准测试由不同的生物医学文本挖掘任务组成。这些任务涵盖了不同的文本类型(生物医学网络数据和临床记录)、数据集大小和难度级别，更重要的是，突出了常见的生物医学文本挖掘挑战。",
			UserID:         123,
			RepositoryType: types.CodeRepo,
			DownloadCount:  10,
			Private:        false,
		},
		{
			Name:           "AAIG_CUP",
			Path:           "MagicAI/AAIG_CUP",
			Nickname:       "电商推荐“抱大腿”攻击识别数据集",
			Description:    "随着互联网的发展，网购成为越来越多人的选择，平台流量竞争也越发激烈。为了保证平台的公平性，如何准确、高效地识别这类型的恶意流量攻击，实时过滤恶意的点击数据是推荐系统中迫切需要解决的问题。",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  19,
			Private:        false,
			Tags: []database.Tag{
				{Name: "relation-extraction"},
				{Name: "zero-shot"},
				{Name: "other"},
			},
		},
		{
			Name:           "Stable",
			Path:           "opencsg/stable-diffusion-3-medium",
			Nickname:       "opencsg",
			Description:    "great model",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
		{
			Name:           "Qwen2.5",
			Path:           "xzgan001/Qwen2.5-Coder-32B-Instruct",
			Nickname:       "Qwen2.5-Coder-32B-Instruct",
			Description:    "Qwen",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
		{
			Name:           "ChatGlm3",
			Path:           "mason/chatglm3-6b-32k",
			Nickname:       "chatglm3",
			Description:    "chatglm3 is a great model",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
		{
			Name:           "Qwen2-1.5B-Instruct-GGUF",
			Path:           "AIWizards/Qwen2-1.5B-Instruct-GGUF",
			Nickname:       "Qwen2-GGUF",
			Description:    "weighted/imatrix quants seem not to be available (by me) at this time. If they do not show up a week or so after the static ones, I have probably not planned for them. Feel free to request them by opening a Community Discussion.",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
		{
			Name:           "Qwen2-1.5B",
			Path:           "Qwen/Qwen2-1.5B",
			Nickname:       "Qwen2-1.5B",
			Description:    "qwen2 is a great model",
			UserID:         123,            // Replace with actual user ID
			RepositoryType: types.CodeRepo, // Replace with appropriate repository type
			DownloadCount:  1,
			Private:        false,
		},
	}

	for _, repo := range repos {
		repo.GitPath = repo.Path
		rn, err := store.CreateRepo(ctx, *repo)
		if err != nil {
			return nil, err
		}
		code := database.Code{
			RepositoryID: rn.ID,
		}
		_, err = db.Core.NewInsert().Model(&code).Exec(ctx)
		if err != nil {
			return nil, err
		}
		for _, tag := range repo.Tags {
			_, err = db.Core.NewInsert().Model(&tag).Exec(ctx, &tag)
			if err != nil {
				return nil, err
			}
			rtags := []database.RepositoryTag{
				{TagID: tag.ID, RepositoryID: rn.ID, Count: 1},
			}
			err = store.BatchCreateRepoTags(ctx, rtags)
			if err != nil {
				return nil, err
			}
		}
	}
	return repos, nil
}

type repoSearchCase struct {
	admin    bool
	repoType types.RepositoryType
	source   string
	search   string
	tags     []types.TagReq
	sort     string
	expected []string
}

func TestRepoStore_PublicToUserSearch_Sqlite(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	redisClient := tests.InitTestRedis()
	defer redisClient.Close()

	config := &config.Config{}
	config.Search.RepoSearchCacheTTL = 300
	config.Search.RepoSearchLimit = 10
	config.Database.Driver = "sqlite"
	store := database.NewRepoStoreWithCache(config, db, cache.NewCacheWithClient(ctx, redisClient))

	_, err := setupTestRepositories(ctx, store, db)
	require.Nil(t, err)
	cases := []repoSearchCase{
		{
			admin: false, repoType: types.CodeRepo,
			search:   "billionaire",
			expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
		},
		// test cache
		{
			admin: false, repoType: types.CodeRepo,
			search:   "billionaire",
			expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
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
			require.ElementsMatch(t, c.expected, names)
			require.Equal(t, len(c.expected), count)
		})
	}
}

func TestRepoStore_PublicToUserSearch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	redisClient := tests.InitTestRedis()
	defer redisClient.Close()

	config := &config.Config{}
	config.Search.RepoSearchCacheTTL = 300
	config.Search.RepoSearchLimit = 10
	config.Database.Driver = "pg"
	config.Database.SearchConfiguration = "opencsgchinese"
	store := database.NewRepoStoreWithCache(config, db, cache.NewCacheWithClient(ctx, redisClient))

	_, err := setupTestRepositories(ctx, store, db)
	require.Nil(t, err)
	hasZhparser, err := tests.CheckZhparser(ctx, db.BunDB, "pg")
	require.NoError(t, err)
	cases := defineTestCases(hasZhparser)

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
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
			require.ElementsMatch(t, c.expected, names)
			require.Equal(t, len(c.expected), count)
		})
	}
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
			sort:     "recently_update",
			expected: []string{"rp6", "rp5", "rp4", "rp2", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo, source: string(types.HuggingfaceSource),
			expected: []string{"rp2"},
		},
		{
			admin: true, repoType: types.CodeRepo,
			sort:     "recently_update",
			expected: []string{"rp6", "rp5", "rp4", "rp3", "rp2", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo, search: "rp4",
			expected: []string{"rp4", "rp6"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			sort:     "most_download",
			expected: []string{"rp6", "rp5", "rp4", "rp2", "rp1"},
		},
		{
			admin: false, repoType: types.CodeRepo,
			sort:     "trending",
			expected: []string{"rp6", "rp5", "rp4", "rp2", "rp1"},
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
			redisClient := tests.InitTestRedis()
			defer redisClient.Close()

			config := &config.Config{}
			config.Search.RepoSearchCacheTTL = 300
			config.Search.RepoSearchLimit = 10
			config.Database.Driver = "pg"
			config.Database.SearchConfiguration = "opencsgchinese"
			store := database.NewRepoStoreWithCache(config, db, cache.NewCacheWithClient(ctx, redisClient))
			recomStore := database.NewRecomStoreWithDB(db)

			repos := []*database.Repository{
				{
					Name: "rp1", Path: "rp1", UserID: 123, RepositoryType: types.CodeRepo,
					DownloadCount: 11,
				},
				{
					Name: "rp2", Path: "rp2", UserID: 123, RepositoryType: types.CodeRepo,
					Private: true, Source: types.HuggingfaceSource,
					DownloadCount: 12,
				},
				{
					Name: "rp3", Path: "rp3", UserID: 456, RepositoryType: types.CodeRepo,
					Private:       true,
					DownloadCount: 15,
				},
				{
					Name: "rp4", Path: "rp4", UserID: 456, RepositoryType: types.CodeRepo,
					Private: false, Tags: []database.Tag{{Name: "foo"}},
					DownloadCount: 13,
				},
				{
					Name: "rp5", Path: "rp5", UserID: 789, RepositoryType: types.CodeRepo,
					Private: false, Tags: []database.Tag{{Name: "bar"}},
					DownloadCount: 14,
				},
				{
					Name: "rp6", Path: "rp6", UserID: 789, RepositoryType: types.CodeRepo,
					Private: false, Description: "rp4desc",
					DownloadCount: 16,
				},
			}

			for _, repo := range repos {
				repo.GitPath = repo.Path
				// update time forcely to make sure the order of repos since the updated_at will not be updated automatically
				repo.UpdatedAt = time.Now()
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

				// Extract number from repo name (e.g., "rp4" -> 4) and multiply by 100
				if strings.HasPrefix(repo.Path, "rp") {
					if numStr := strings.TrimPrefix(repo.Path, "rp"); numStr != "" {
						if num, err := strconv.Atoi(numStr); err == nil {
							// insert recom repo score
							err := recomStore.UpsertScore(ctx, []*database.RecomRepoScore{
								{
									RepositoryID: rn.ID,
									WeightName:   database.RecomWeightTotal,
									Score:        float64(num * 100),
								},
							})
							require.Nil(t, err)
						}
					}
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
		GitPath:        "codes_ns/n",
		RepositoryType: types.CodeRepo,
		Path:           "ns/n",
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

func TestRepoStore_ListRepoByDeployType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	rn1, err := store.CreateRepo(ctx, database.Repository{
		ID:             1,
		UserID:         123,
		Path:           "ns/n",
		GitPath:        "models_ns/n",
		RepositoryType: types.ModelRepo,
		Private:        true,
	})
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID: rn1.ID,
		Architecture: "Qwen2ForCausalLM",
		ClassName:    "qwen2",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID: 2,
		Architecture: "Qwen3ForCausalLM",
		ClassName:    "qwen3",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.Metadata{
		RepositoryID: 3,
		Architecture: "Qwen4ForCausalLM",
		ClassName:    "qwen4",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RuntimeFramework{
		ID:          1,
		FrameName:   "vllm",
		Type:        1,
		ModelFormat: "safetensors",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RuntimeFramework{
		ID:          2,
		FrameName:   "llama.cpp",
		Type:        1,
		ModelFormat: "gguf",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RuntimeArchitecture{
		ArchitectureName:   "Qwen2ForCausalLM",
		RuntimeFrameworkID: 1,
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RuntimeArchitecture{
		ArchitectureName:   "qwen3",
		RuntimeFrameworkID: 1,
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = store.CreateRepo(ctx, database.Repository{
		ID:             2,
		UserID:         123,
		Path:           "ns/n2",
		GitPath:        "models_ns/n2",
		RepositoryType: types.ModelRepo,
		Private:        false,
	})
	require.Nil(t, err)
	_, err = store.CreateRepo(ctx, database.Repository{
		ID:             3,
		UserID:         123,
		Path:           "ns/n3",
		GitPath:        "models_ns/n3",
		RepositoryType: types.ModelRepo,
		Private:        false,
	})
	require.Nil(t, err)
	rs, count, err := store.ListRepoByDeployType(ctx, types.ModelRepo, 123, "", "", 1, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, "ns/n", rs[0].Path)

}

func TestRepoStore_CleanRelationsByRepoID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	rn, err := store.CreateRepo(ctx, database.Repository{
		GitPath:        "codes_ns/n",
		Path:           "ns/n",
		RepositoryType: types.CodeRepo,
	})
	require.Nil(t, err)

	rn1, err := store.CreateRepo(ctx, database.Repository{
		GitPath:        "codes_ns/n1",
		Path:           "ns/n1",
		RepositoryType: types.CodeRepo,
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

	var mirror database.Mirror
	err = db.Core.NewInsert().Model(&database.Mirror{RepositoryID: rn.ID}).Scan(ctx, &mirror)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.MirrorTask{MirrorID: mirror.ID}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.LfsMetaObject{RepositoryID: rn.ID, Oid: "oid"}).Exec(ctx)
	require.Nil(t, err)

	count, err = db.Core.NewSelect().Model(&database.Mirror{}).Where("repository_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.MirrorTask{}).Where("mirror_id=?", mirror.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.LfsMetaObject{}).Where("repository_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	var mirror1 database.Mirror
	err = db.Core.NewInsert().Model(&database.Mirror{RepositoryID: rn1.ID}).Scan(ctx, &mirror1)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.MirrorTask{MirrorID: mirror1.ID}).Exec(ctx)
	require.Nil(t, err)

	count, err = db.Core.NewSelect().Model(&database.Mirror{}).Where("repository_id=?", rn1.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.MirrorTask{}).Where("mirror_id=?", mirror1.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	err = store.CleanRelationsByRepoID(ctx, rn.ID)
	require.Nil(t, err)

	count, err = db.Core.NewSelect().Model(&database.LfsMetaObject{}).Where("repository_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)

	count, err = db.Core.NewSelect().Model(&database.Mirror{}).Where("repository_id=?", rn.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)

	count, err = db.Core.NewSelect().Model(&database.MirrorTask{}).Where("mirror_id=?", mirror.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, count)

	count, err = db.Core.NewSelect().Model(&database.Mirror{}).Where("repository_id=?", rn1.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

	count, err = db.Core.NewSelect().Model(&database.MirrorTask{}).Where("mirror_id=?", mirror1.ID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, count)

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
	// Test with pending filter
	pendingStatus := types.SensitiveCheckPending
	pendingFilter := &types.BatchGetFilter{
		RepoType:             types.CodeRepo,
		SensitiveCheckStatus: &pendingStatus,
	}
	rs, err := store.BatchGet(ctx, 0, 10, pendingFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 3)
	require.Equal(t, []string{"rp1", "rp2", "rp3"}, names(rs))

	rs, err = store.BatchGet(ctx, rids[1], 10, pendingFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.Equal(t, []string{"rp3"}, names(rs))

	rs, err = store.BatchGet(ctx, 0, 1, pendingFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.Equal(t, []string{"rp1"}, names(rs))

	// Test with pass filter (should return CodeRepo with Pass status)
	passStatus := types.SensitiveCheckPass
	passFilter := &types.BatchGetFilter{
		RepoType:             types.CodeRepo,
		SensitiveCheckStatus: &passStatus,
	}
	rs, err = store.BatchGet(ctx, 0, 10, passFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1) // Should return rp4 which is CodeRepo with Pass status
	require.Equal(t, []string{"rp4"}, names(rs))

	// Test with different status for DatasetRepo
	pendingStatus2 := types.SensitiveCheckPending
	datasetFilter := &types.BatchGetFilter{
		RepoType:             types.DatasetRepo,
		SensitiveCheckStatus: &pendingStatus2,
	}
	rs, err = store.BatchGet(ctx, 0, 10, datasetFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.Equal(t, []string{"rp5"}, names(rs))

	// Test with nil filter (should return all repos)
	rs, err = store.BatchGet(ctx, 0, 10, nil)
	require.Nil(t, err)
	require.Equal(t, len(rs), 5) // Should return all 5 repos
	require.Equal(t, []string{"rp1", "rp2", "rp3", "rp4", "rp5"}, names(rs))

	// Test with empty filter (should return all repos)
	emptyFilter := &types.BatchGetFilter{}
	rs, err = store.BatchGet(ctx, 0, 10, emptyFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 5) // Should return all 5 repos
	require.Equal(t, []string{"rp1", "rp2", "rp3", "rp4", "rp5"}, names(rs))

	// Test with only RepoType filter
	repoTypeOnlyFilter := &types.BatchGetFilter{
		RepoType: types.CodeRepo,
	}
	rs, err = store.BatchGet(ctx, 0, 10, repoTypeOnlyFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 4) // Should return 4 CodeRepo repos
	require.Equal(t, []string{"rp1", "rp2", "rp3", "rp4"}, names(rs))

	// Test with only SensitiveCheckStatus filter
	pendingStatus3 := types.SensitiveCheckPending
	statusOnlyFilter := &types.BatchGetFilter{
		SensitiveCheckStatus: &pendingStatus3,
	}
	rs, err = store.BatchGet(ctx, 0, 10, statusOnlyFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 4) // Should return 4 repos with pending status
	require.Equal(t, []string{"rp1", "rp2", "rp3", "rp5"}, names(rs))

	// Test with SensitiveCheckStatus = 0 (Pending) to ensure 0 is treated as a valid filter value
	pendingStatusZero := types.SensitiveCheckStatus(0) // Explicitly set to 0 (Pending)
	statusZeroFilter := &types.BatchGetFilter{
		SensitiveCheckStatus: &pendingStatusZero,
	}
	rs, err = store.BatchGet(ctx, 0, 10, statusZeroFilter)
	require.Nil(t, err)
	require.Equal(t, len(rs), 4) // Should return 4 repos with pending status (0)
	require.Equal(t, []string{"rp1", "rp2", "rp3", "rp5"}, names(rs))

	rs, err = store.FindWithBatch(ctx, 2, 1)
	require.Nil(t, err)
	require.Equal(t, len(rs), 2)
	require.Equal(t, []string{"rp3", "rp2"}, names(rs))

	rs, err = store.FindWithBatch(ctx, 2, 0, types.DatasetRepo)
	require.Nil(t, err)
	require.Equal(t, len(rs), 1)
	require.Equal(t, []string{"rp5"}, names(rs))

	rs, err = store.ByUser(ctx, 123, 100, 0)
	require.Nil(t, err)
	require.Equal(t, len(rs), 3)
	require.ElementsMatch(t, []string{"rp1", "rp2", "rp3"}, names(rs))

	rs, err = store.FindByRepoSourceWithBatch(ctx, types.HuggingfaceSource, 2, 1)
	require.Nil(t, err)
	require.Equal(t, len(rs), 2)
	require.Equal(t, []string{"rp2", "rp1"}, names(rs))
}

func TestRepoStore_FindMirrorReposByUserAndSource(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repoStore := database.NewRepoStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	mirrorSouceStore := database.NewMirrorSourceStoreWithDB(db)

	namespace := &database.Namespace{
		Path: "test",
	}
	user := &database.User{
		Username: "test",
	}
	err := userStore.Create(ctx, user, namespace)
	require.Nil(t, err)

	repo := database.Repository{
		Path:           "test/test",
		GitPath:        "test/test",
		Name:           "test",
		Nickname:       "test",
		Private:        false,
		DefaultBranch:  "main",
		RepositoryType: types.CodeRepo,
		UserID:         user.ID,
	}
	repoRes, err := repoStore.CreateRepo(ctx, repo)
	require.Nil(t, err)

	mirrorSource := &database.MirrorSource{
		SourceName: "gitlab",
	}
	_, err = mirrorSouceStore.Create(ctx, mirrorSource)
	require.Nil(t, err)

	mirror := &database.Mirror{
		RepositoryID:   repoRes.ID,
		MirrorSourceID: mirrorSource.ID,
	}
	_, err = mirrorStore.Create(ctx, mirror)
	require.Nil(t, err)

	repos, err := repoStore.FindMirrorReposByUserAndSource(ctx, user.ID, "gitlab", 10, 1)
	require.Nil(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, repoRes.ID, repos[0].ID)
}

func TestRepoStore_UpdateSourcePath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repoStore := database.NewRepoStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)

	namespace := &database.Namespace{
		Path: "test",
	}
	user := &database.User{
		Username: "test",
	}
	err := userStore.Create(ctx, user, namespace)
	require.Nil(t, err)

	repo := database.Repository{
		Path:           "test/test",
		GitPath:        "test/test",
		Name:           "test",
		Nickname:       "test",
		Private:        false,
		DefaultBranch:  "main",
		RepositoryType: types.CodeRepo,
		UserID:         user.ID,
	}
	repoRes, err := repoStore.CreateRepo(ctx, repo)
	require.Nil(t, err)

	err = repoStore.UpdateSourcePath(ctx, repoRes.ID, "abc/def", "opencsg")
	require.Nil(t, err)
	err = repoStore.UpdateSourcePath(ctx, repoRes.ID, "aaa/bbb", "huggingface")
	require.Nil(t, err)
	err = repoStore.UpdateSourcePath(ctx, repoRes.ID, "ccc/ddd", "modelscope")
	require.Nil(t, err)
	repoFinal, err := repoStore.FindById(ctx, repoRes.ID)
	require.Nil(t, err)
	require.Equal(t, repoFinal.CSGPath, "abc/def")
	require.Equal(t, repoFinal.HFPath, "aaa/bbb")
	require.Equal(t, repoFinal.MSPath, "ccc/ddd")
}

func TestRepoStore_FindMirrorReposWithBatch(t *testing.T) {
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
	rs, err := store.FindMirrorReposWithBatch(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(rs))
	require.Equal(t, "codes_ns/n", rs[0].GitPath)
}

func TestRepoStore_BulkUpdateSourcePath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	var repos []*database.Repository
	store := database.NewRepoStoreWithDB(db)
	rn, err := store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns/n",
	})
	require.Nil(t, err)

	rn2, err := store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns2/n",
		Path:    "zzz",
	})
	require.Nil(t, err)

	rn.CSGPath = "abc/def"
	rn2.MSPath = "aaa/bbb"

	repos = append(repos, rn, rn2)
	err = store.BulkUpdateSourcePath(ctx, repos)
	require.Nil(t, err)

	rn, err = store.FindById(ctx, rn.ID)
	require.Nil(t, err)
	require.Equal(t, "abc/def", rn.CSGPath)

	rn2, err = store.FindById(ctx, rn2.ID)
	require.Nil(t, err)
	require.Equal(t, "aaa/bbb", rn2.MSPath)
}

func TestRepoStore_FindByMirrorSourceURL(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	sourceUrl := "https://github.com/codes_ns/n"
	store := database.NewRepoStoreWithDB(db)
	rn, err := store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns/n",
	})

	require.Nil(t, err)

	mirrorStore := database.NewMirrorStoreWithDB(db)
	_, err = mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:    sourceUrl,
		RepositoryID: rn.ID,
	})

	require.Nil(t, err)

	rn, err = store.FindByMirrorSourceURL(ctx, sourceUrl)
	require.Nil(t, err)
	require.Equal(t, "codes_ns/n", rn.GitPath)
}

func TestRepoStore_RefreshLFSObjectsSize(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	rn, err := store.CreateRepo(ctx, database.Repository{
		GitPath: "codes_ns/n",
	})

	require.Nil(t, err)

	lfsStore := database.NewLfsMetaObjectStoreWithDB(db)
	err = lfsStore.BulkUpdateOrCreate(ctx, rn.ID, []database.LfsMetaObject{
		{
			RepositoryID: rn.ID,
			Size:         12,
			Oid:          "123",
		},
		{
			RepositoryID: rn.ID,
			Size:         10,
			Oid:          "321",
		},
	})

	require.Nil(t, err)

	err = store.RefreshLFSObjectsSize(ctx, rn.ID)
	require.Nil(t, err)

	repo, err := store.FindById(ctx, rn.ID)
	require.Nil(t, err)
	require.Equal(t, int64(22), repo.LFSObjectsSize)
}

func TestRepoStore_FindMirrorFinishedPrivateModelRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	r, err := store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n",
		Path:                 "ns/n",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	})

	require.Nil(t, err)

	r1, err := store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n1",
		Path:                 "ns/n1",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	})
	require.Nil(t, err)

	r2, err := store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n2",
		Path:                 "ns/n2",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckFail,
		Private:              true,
	})
	require.Nil(t, err)

	mirrorStore := database.NewMirrorStoreWithDB(db)
	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		RepositoryID: r.ID,
		Status:       types.MirrorLfsSyncFinished,
	})

	require.Nil(t, err)

	_, err = mirrorStore.Create(ctx, &database.Mirror{
		RepositoryID: r1.ID,
		Status:       types.MirrorLfsSyncFinished,
	})

	require.Nil(t, err)

	_, err = mirrorStore.Create(ctx, &database.Mirror{
		RepositoryID: r2.ID,
		Status:       types.MirrorLfsSyncFinished,
	})

	require.Nil(t, err)

	mirrorTaskStore := database.NewMirrorTaskStoreWithDB(db)
	_, err = mirrorTaskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorLfsSyncFinished,
	})
	require.Nil(t, err)

	repos, err := store.FindMirrorFinishedPrivateModelRepo(ctx)
	require.Nil(t, err)

	require.Equal(t, 1, len(repos))
	require.Equal(t, "codes_ns/n", repos[0].GitPath)
}

func TestRepoStore_BatchUpdate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	r, err := store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n",
		Path:                 "ns/n",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	})

	require.Nil(t, err)

	r1, err := store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n1",
		Path:                 "ns/n1",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	})
	require.Nil(t, err)

	r2, err := store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n2",
		Path:                 "ns/n2",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckFail,
		Private:              true,
	})
	require.Nil(t, err)

	r.Private = false
	r1.Private = false
	r2.Private = false

	err = store.BatchUpdate(ctx, []*database.Repository{r, r1, r2})
	require.Nil(t, err)

	repos, err := store.FindByIds(ctx, []int64{r.ID, r1.ID, r2.ID})
	require.Nil(t, err)

	require.Equal(t, 3, len(repos))
	require.Equal(t, false, repos[0].Private)
	require.Equal(t, false, repos[1].Private)
	require.Equal(t, false, repos[2].Private)
}

func TestRepoStore_FindByRepoTypeAndPaths(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoStoreWithDB(db)
	_, err := store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n",
		Path:                 "ns/n",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	})

	require.Nil(t, err)

	_, err = store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n1",
		Path:                 "ns/n1",
		RepositoryType:       types.ModelRepo,
		SensitiveCheckStatus: types.SensitiveCheckPass,
		Private:              true,
	})
	require.Nil(t, err)

	_, err = store.CreateRepo(ctx, database.Repository{
		GitPath:              "codes_ns/n2",
		Path:                 "ns/n2",
		RepositoryType:       types.DatasetRepo,
		SensitiveCheckStatus: types.SensitiveCheckFail,
		Private:              true,
	})
	require.Nil(t, err)

	repos, err := store.FindByRepoTypeAndPaths(ctx, types.ModelRepo, []string{"ns/n", "ns/n1", "ns/n2"})
	require.Nil(t, err)

	require.Equal(t, 2, len(repos))
	actualPaths := make([]string, len(repos))
	for i, r := range repos {
		actualPaths[i] = r.GitPath
	}
	require.ElementsMatch(t, []string{"codes_ns/n", "codes_ns/n1"}, actualPaths)
}

func TestRepoStore_PublicToUserMirror(t *testing.T) {
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
		Source:         "local",
	})
	require.Nil(t, err)
	require.NotNil(t, repo)

	userName1 := "user_name_" + uuid.NewString()
	repoName1 := "repo_name_" + uuid.NewString()
	repo1, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName1, repoName1),
		GitPath:        fmt.Sprintf("datasets_%s/%s", userName1, repoName1),
		Name:           repoName,
		DefaultBranch:  "main",
		Nickname:       "ww",
		Description:    "ww",
		Private:        false,
		RepositoryType: types.DatasetRepo,
		Source:         "opencsg",
	})
	require.Nil(t, err)
	require.NotNil(t, repo1)

	mirrorStore := database.NewMirrorStoreWithDB(db)
	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		RepositoryID: repo1.ID,
	})
	require.Nil(t, err)

	mirrorTaskStore := database.NewMirrorTaskStoreWithDB(db)
	_, err = mirrorTaskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorLfsSyncFinished,
	})
	require.Nil(t, err)

	filter := &types.RepoFilter{
		Source: "local",
	}
	// case 1: one tag
	repos, _, err := rs.PublicToUser(ctx, repo.RepositoryType, []int64{1}, filter, 20, 1, true)
	require.Nil(t, err)
	require.NotNil(t, repos)
	require.Equal(t, 2, len(repos))
}

func TestRepoStore_FindUnhashedRepos(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// insert a new repo
	rs := database.NewRepoStoreWithDB(db)
	for i := 0; i < 10; i++ {
		_, err := db.Operator.Core.NewInsert().Model(&database.Repository{
			UserID:         1,
			Path:           fmt.Sprintf("%s/%d", "path", i),
			GitPath:        fmt.Sprintf("datasets_%s/%d", "path", i),
			Name:           fmt.Sprintf("name_%d", i),
			DefaultBranch:  "main",
			Nickname:       "ww",
			Description:    "ww",
			Private:        false,
			RepositoryType: types.DatasetRepo,
			Source:         "opencsg",
			Hashed:         false,
		}).Exec(ctx)
		require.Nil(t, err)
	}

	userName1 := "user_name_" + uuid.NewString()
	repoName1 := "repo_name_" + uuid.NewString()
	repo1, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName1, repoName1),
		GitPath:        fmt.Sprintf("datasets_%s/%s", userName1, repoName1),
		Name:           repoName1,
		DefaultBranch:  "main",
		Nickname:       "ww",
		Description:    "ww",
		Private:        false,
		RepositoryType: types.DatasetRepo,
		Source:         "opencsg",
	})
	require.Nil(t, err)
	require.NotNil(t, repo1)

	repos, err := rs.FindUnhashedRepos(ctx, 10, 0)
	require.Nil(t, err)
	require.NotNil(t, repos)
	require.Equal(t, 10, len(repos))
	for _, repo := range repos {
		require.Equal(t, false, repo.Hashed)
	}
}

func TestRepoStore_PublicToUserWithCacheFailed(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().Exists(mock.Anything, mock.Anything).Return(0, nil).Once()
	cache.EXPECT().ZAdd(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("error")).Once()

	config := &config.Config{}
	config.Search.RepoSearchCacheTTL = 300
	config.Search.RepoSearchLimit = 10
	config.Database.Driver = "pg"
	config.Database.SearchConfiguration = "opencsgchinese"

	store := database.NewRepoStoreWithCache(config, db, cache)
	_, err := setupTestRepositories(ctx, store, db)
	require.Nil(t, err)

	c := repoSearchCase{

		admin:    false,
		repoType: types.CodeRepo,
		search:   "billionaire",
		expected: []string{"ChineseBlue", "ChineseMedicalBooksCollection"},
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
	require.ElementsMatch(t, c.expected, names)
	require.Equal(t, len(c.expected), count)
}

func TestRepoStore_UpdateRepoSensitiveCheckStatus(t *testing.T) {
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

	err = store.UpdateRepoSensitiveCheckStatus(ctx, repo.ID, types.SensitiveCheckPass)
	require.Nil(t, err)

	rp := &database.Repository{}
	err = db.Core.NewSelect().Model(rp).Where("id=?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SensitiveCheckPass, rp.SensitiveCheckStatus)
}

func TestRepoStore_GetReposBySearch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	// insert a new repo
	rs := database.NewRepoStoreWithDB(db)
	for i := 0; i < 10; i++ {
		_, err := db.Operator.Core.NewInsert().Model(&database.Repository{
			UserID:         1,
			Path:           fmt.Sprintf("%s/%d", "path", i),
			GitPath:        fmt.Sprintf("datasets_%s/%d", "path", i),
			Name:           fmt.Sprintf("name_%d", i),
			DefaultBranch:  "main",
			Nickname:       "ww",
			Description:    "ww",
			Private:        false,
			RepositoryType: types.DatasetRepo,
			Source:         "opencsg",
			Hashed:         false,
		}).Exec(ctx)
		require.Nil(t, err)
	}

	repos, total, err := rs.GetReposBySearch(ctx, "path/1", types.DatasetRepo, 1, 10)
	require.Nil(t, err)
	require.NotNil(t, repos)
	require.Equal(t, 1, total)
}
