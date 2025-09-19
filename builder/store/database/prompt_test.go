package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestPromptStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := database.NewPromptStoreWithDB(db)

	p := database.Prompt{
		RepositoryID: 1234,
	}

	_, err := ps.Create(ctx, p)
	require.Nil(t, err)

	_, err = ps.ByRepoID(ctx, 1234)
	require.Nil(t, err)

}

func TestPromptStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := database.NewPromptStoreWithDB(db)

	_, err := ps.Create(ctx, database.Prompt{
		RepositoryID: 1234,
	})
	require.Nil(t, err)

	p, err := ps.ByRepoID(ctx, 1234)
	require.Nil(t, err)
	p.RepositoryID = 3456
	err = ps.Update(ctx, *p)
	require.Nil(t, err)

	_, err = ps.ByRepoID(ctx, 1234)
	require.NotNil(t, err)

	_, err = ps.ByRepoID(ctx, 3456)
	require.Nil(t, err)

}

func TestPromptStore_ByRepoID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := database.NewPromptStoreWithDB(db)

	p := database.Prompt{
		RepositoryID: 1234,
	}

	_, err := ps.Create(ctx, p)
	require.Nil(t, err)

	_, err = ps.ByRepoID(ctx, 1234)
	require.Nil(t, err)
}

func TestPromptStore_ByRepoIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := database.NewPromptStoreWithDB(db)
	rs := database.NewRepoStoreWithDB(db)
	us := database.NewUserStoreWithDB(db)

	err := us.Create(ctx, &database.User{
		Username: "foo",
	}, &database.Namespace{})
	require.Nil(t, err)
	user, err := us.FindByUsername(ctx, "foo")
	require.Nil(t, err)

	repoIds := []int64{}
	for _, r := range []string{"a", "b", "c"} {

		repo, err := rs.CreateRepo(ctx, database.Repository{
			UserID:  user.ID,
			Name:    r,
			Path:    r,
			GitPath: r,
		})
		require.Nil(t, err)
		repoIds = append(repoIds, repo.ID)

		_, err = ps.Create(ctx, database.Prompt{
			RepositoryID: repo.ID,
		})

		require.Nil(t, err)
	}

	prompts, err := ps.ByRepoIDs(ctx, repoIds)
	require.Nil(t, err)

	require.Equal(t, 3, len(prompts))
	names := []string{}
	for _, p := range prompts {
		require.Equal(t, "foo", p.Repository.User.Username)
		names = append(names, p.Repository.Name)
	}
	require.ElementsMatch(t, []string{"a", "b", "c"}, names)
}

func TestPromptStore_FindByPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	// db.BunDB.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rs := database.NewRepoStoreWithDB(db)
	ts := database.NewTagStoreWithDB(db)

	repo, err := rs.CreateRepo(ctx, database.Repository{
		Path:    "a/b/c",
		GitPath: "abc",
	})
	require.Nil(t, err)

	tags := []database.RepositoryTag{}
	// tab a,c count -1, tag b count 1
	for _, n := range []string{"a", "b", "c"} {
		var c int32 = -1
		if n == "b" {
			c = 1
		}
		tag := database.Tag{
			Category: "foo",
			Name:     n,
			Group:    n,
			Scope:    types.DatasetTagScope,
		}
		dbTag, err := ts.CreateTag(ctx, tag)
		require.Nil(t, err)
		tags = append(tags, database.RepositoryTag{
			RepositoryID: repo.ID,
			TagID:        dbTag.ID,
			Count:        c,
		})
	}

	err = rs.BatchCreateRepoTags(ctx, tags)
	require.Nil(t, err)

	ps := database.NewPromptStoreWithDB(db)
	_, err = ps.Create(ctx, database.Prompt{
		RepositoryID: repo.ID,
	})
	require.Nil(t, err)

	prompt, err := ps.FindByPath(ctx, "a", "b/c")
	require.Nil(t, err)
	require.Equal(t, "", prompt.Repository.Name, "abc")
	// FindByPath only get tags with count > 0 (tag b)
	require.Equal(t, 1, len(prompt.Repository.Tags))
	require.Equal(t, prompt.Repository.Tags[0].Name, "b")

}

func TestPromptStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ps := database.NewPromptStoreWithDB(db)

	err := ps.Delete(ctx, database.Prompt{ID: 123})
	require.NotNil(t, err)

	p, err := ps.Create(ctx, database.Prompt{})
	require.Nil(t, err)

	err = ps.Delete(ctx, *p)
	require.Nil(t, err)

	err = ps.Delete(ctx, *p)
	require.NotNil(t, err)

}

func TestPromptStore_ByUserName(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	us := database.NewUserStoreWithDB(db)
	err := us.Create(ctx, &database.User{
		Username: "foo",
	}, &database.Namespace{})
	require.Nil(t, err)
	user, err := us.FindByUsername(ctx, "foo")
	require.Nil(t, err)

	rs := database.NewRepoStoreWithDB(db)

	// order: 4-6-2-1-3-5
	repos := []struct {
		Name      string
		CreatedAt time.Time
		Private   bool
	}{
		{"repo1", time.Unix(1731561102, 0), false},
		{"repo2", time.Unix(1731561302, 0), false},
		{"repo3", time.Unix(1731551102, 0), false},
		{"repo4", time.Unix(1731564132, 0), true},
		{"repo5", time.Unix(1721561102, 0), false},
		{"repo6", time.Unix(1731564102, 0), true},
	}

	ps := database.NewPromptStoreWithDB(db)
	for _, repo := range repos {
		r := database.Repository{
			UserID:  user.ID,
			Name:    repo.Name,
			Path:    repo.Name,
			GitPath: repo.Name,
			Private: repo.Private,
		}

		rp, err := rs.CreateRepo(ctx, r)
		require.Nil(t, err)

		pm := database.Prompt{
			RepositoryID: rp.ID,
		}
		pm.CreatedAt = repo.CreatedAt
		_, err = ps.Create(ctx, pm)
		require.Nil(t, err)
	}

	cases := []struct {
		per        int
		page       int
		total      int
		onlyPublic bool
		expected   []int
	}{
		{10, 1, 6, false, []int{4, 6, 2, 1, 3, 5}},
		{10, 1, 4, true, []int{2, 1, 3, 5}},
		{2, 1, 6, false, []int{4, 6}},
		{2, 2, 6, false, []int{2, 1}},
		{2, 1, 4, true, []int{2, 1}},
		{2, 2, 4, true, []int{3, 5}},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("page %d, per %d, public %t", c.page, c.per, c.onlyPublic), func(t *testing.T) {
			prompts, total, err := ps.ByUsername(ctx, "foo", c.per, c.page, c.onlyPublic)
			require.Nil(t, err)
			names := []string{}
			for _, pm := range prompts {
				names = append(names, pm.Repository.Name)
			}
			expected := []string{}
			for _, i := range c.expected {
				expected = append(expected, fmt.Sprintf("repo%d", i))
			}

			require.Equal(t, c.total, total)
			require.Equal(t, expected, names)

		})
	}

}

func TestPromptStore_CreateAndUpdateRepoPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPromptStoreWithDB(db)

	repo := &database.Repository{
		Name: "repo1",
		Path: "p1",
	}
	err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	prompt, err := store.CreateAndUpdateRepoPath(ctx, database.Prompt{
		RepositoryID: repo.ID,
	}, "p2")

	require.Nil(t, err)
	require.Equal(t, "p2", prompt.Repository.Path)
}
