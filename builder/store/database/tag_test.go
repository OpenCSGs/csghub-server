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

func TestTagStore_FindOrCreate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := database.NewTagStoreWithDB(db)
	uuid := uuid.New().String()
	tag := database.Tag{
		Name:     "tag_" + uuid,
		Category: "task",
		Group:    "",
		Scope:    "model",
		BuiltIn:  true,
		ShowName: "Tag One",
	}
	newTag, err := ts.FindOrCreate(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, newTag.ID)
	require.Equal(t, tag.Name, newTag.Name)

	existingTag, err := ts.FindOrCreate(ctx, tag)
	require.Empty(t, err)
	require.Equal(t, newTag.ID, existingTag.ID)
}

func TestTagStore_AllTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	//clear all existing data
	_, err = db.Core.NewDelete().Model(&database.Tag{}).Where("1=1").Exec(ctx)
	require.Empty(t, err)

	ts := database.NewTagStoreWithDB(db)
	tags, err := ts.AllTags(ctx, nil)
	require.Empty(t, err)
	require.Equal(t, 0, len(tags))

	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "Group One",
		Scope:    types.ModelTagScope,
	}
	modelTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	tag = database.Tag{
		Category: "library",
		Name:     "tag_" + uuid.New().String(),
		Group:    "Group One",
		Scope:    types.DatasetTagScope,
	}
	datasetTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	tag = database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "Group One",
		Scope:    types.CodeTagScope,
	}
	codeTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	tag = database.Tag{
		Category: "library",
		Name:     "tag_" + uuid.New().String(),
		Group:    "Group One",
		Scope:    types.PromptTagScope,
	}
	promptTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	tag = database.Tag{
		Category: "library",
		Name:     "tag_" + uuid.New().String(),
		Group:    "Group One",
		Scope:    types.SpaceTagScope,
	}
	spaceTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	tag = database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "Group One",
		Scope:    types.MCPTagScope,
	}
	mcpTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	tags, err = ts.AllTags(ctx, nil)
	require.Empty(t, err)
	require.Equal(t, 6, len(tags))

	builtIn := false
	filter := &types.TagFilter{
		Scopes:     []types.TagScope{types.ModelTagScope, types.DatasetTagScope, types.CodeTagScope, types.PromptTagScope, types.SpaceTagScope},
		Categories: []string{"task"},
		BuiltIn:    &builtIn,
	}
	tags, err = ts.AllTags(ctx, filter)
	require.Empty(t, err)
	require.Equal(t, 2, len(tags))

	filterSearch := &types.TagFilter{
		Scopes:     []types.TagScope{types.ModelTagScope, types.DatasetTagScope, types.CodeTagScope, types.PromptTagScope, types.SpaceTagScope},
		Categories: []string{"task"},
		BuiltIn:    &builtIn,
		Search:     "search",
	}

	tags, err = ts.AllTags(ctx, filterSearch)
	require.Empty(t, err)
	require.Equal(t, 1, len(tags))

	filterSearchNoResult := &types.TagFilter{
		Scopes:     []types.TagScope{types.ModelTagScope, types.DatasetTagScope, types.CodeTagScope, types.PromptTagScope, types.SpaceTagScope},
		Categories: []string{"task"},
		BuiltIn:    &builtIn,
		Search:     "noResult",
	}

	tags, err = ts.AllTags(ctx, filterSearchNoResult)
	require.Empty(t, err)
	require.Equal(t, 0, len(tags))

	modelTags, err := ts.AllModelTags(ctx)
	require.Empty(t, err)
	require.Equal(t, 1, len(modelTags))
	require.Equal(t, modelTag.Name, modelTags[0].Name)

	datasetTags, err := ts.AllDatasetTags(ctx)
	require.Empty(t, err)
	require.Equal(t, 1, len(datasetTags))
	require.Equal(t, datasetTag.Name, datasetTags[0].Name)

	codeTags, err := ts.AllCodeTags(ctx)
	require.Empty(t, err)
	require.Equal(t, 1, len(codeTags))
	require.Equal(t, codeTag.Name, codeTags[0].Name)

	promptTags, err := ts.AllPromptTags(ctx)
	require.Empty(t, err)
	require.Equal(t, 1, len(promptTags))
	require.Equal(t, promptTag.Name, promptTags[0].Name)

	spaceTags, err := ts.AllSpaceTags(ctx)
	require.Empty(t, err)
	require.Equal(t, 1, len(spaceTags))
	require.Equal(t, spaceTag.Name, spaceTags[0].Name)

	mcpTags, err := ts.AllMCPTags(ctx)
	require.Empty(t, err)
	require.Equal(t, 1, len(mcpTags))
	require.Equal(t, mcpTag.Name, mcpTags[0].Name)
}

// TestAllModelCategories tests the AllModelCategories method
func TestTagStore_AllModelCategories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	//clear all existing data
	_, err = db.Core.NewDelete().Model(&database.TagCategory{}).Where("1=1").Exec(ctx)
	require.Empty(t, err)

	ts := database.NewTagStoreWithDB(db)
	tags, err := ts.AllModelCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 0)

	tc1 := &database.TagCategory{
		Name:  "tc_1",
		Scope: types.ModelTagScope,
	}
	tc2 := &database.TagCategory{
		Name:  "tc_2",
		Scope: types.DatasetTagScope,
	}
	_, err = db.Core.NewInsert().Model(tc1).Exec(ctx)
	require.Empty(t, err)
	_, err = db.Core.NewInsert().Model(tc2).Exec(ctx)
	require.Empty(t, err)

	tags, err = ts.AllModelCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 1)
}

// TestAllPromptCategories tests the AllPromptCategories method
func TestTagStore_AllPromptCategories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	//clear all existing data
	_, err = db.Core.NewDelete().Model(&database.TagCategory{}).Where("1=1").Exec(ctx)
	require.Empty(t, err)

	ts := database.NewTagStoreWithDB(db)
	tags, err := ts.AllPromptCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 0)

	tc1 := &database.TagCategory{
		Name:  "tc_1",
		Scope: types.ModelTagScope,
	}
	tc2 := &database.TagCategory{
		Name:  "tc_2",
		Scope: types.PromptTagScope,
	}
	_, err = db.Core.NewInsert().Model(tc1).Exec(ctx)
	require.Empty(t, err)
	_, err = db.Core.NewInsert().Model(tc2).Exec(ctx)
	require.Empty(t, err)

	tags, err = ts.AllPromptCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 1)
}

// TestAllDatasetCategories tests the AllDatasetCategories method
func TestTagStore_AllDatasetCategories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	//clear all existing data
	_, err = db.Core.NewDelete().Model(&database.TagCategory{}).Where("1=1").Exec(ctx)
	require.Empty(t, err)

	ts := database.NewTagStoreWithDB(db)
	tags, err := ts.AllDatasetCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 0)

	tc1 := &database.TagCategory{
		Name:  "tc_1",
		Scope: types.ModelTagScope,
	}
	tc2 := &database.TagCategory{
		Name:  "tc_2",
		Scope: types.DatasetTagScope,
	}
	_, err = db.Core.NewInsert().Model(tc1).Exec(ctx)
	require.Empty(t, err)
	_, err = db.Core.NewInsert().Model(tc2).Exec(ctx)
	require.Empty(t, err)

	tags, err = ts.AllDatasetCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 1)
}

// TestAllCodeCategories tests the AllCodeCategories method
func TestTagStore_AllCodeCategories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	//clear all existing data
	_, err = db.Core.NewDelete().Model(&database.TagCategory{}).Where("1=1").Exec(ctx)
	require.Empty(t, err)

	ts := database.NewTagStoreWithDB(db)
	tags, err := ts.AllCodeCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 0)

	tc1 := &database.TagCategory{
		Name:  "tc_1",
		Scope: types.ModelTagScope,
	}
	tc2 := &database.TagCategory{
		Name:  "tc_2",
		Scope: types.CodeTagScope,
	}
	_, err = db.Core.NewInsert().Model(tc1).Exec(ctx)
	require.Empty(t, err)
	_, err = db.Core.NewInsert().Model(tc2).Exec(ctx)
	require.Empty(t, err)

	tags, err = ts.AllCodeCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 1)
}

// TestAllSpaceCategories tests the AllSpaceCategories method
func TestTagStore_AllSpaceCategories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	//clear all existing data
	_, err = db.Core.NewDelete().Model(&database.TagCategory{}).Where("1=1").Exec(ctx)
	require.Empty(t, err)

	ts := database.NewTagStoreWithDB(db)
	tags, err := ts.AllSpaceCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 0)

	tc1 := &database.TagCategory{
		Name:  "tc_1",
		Scope: types.ModelTagScope,
	}
	tc2 := &database.TagCategory{
		Name:  "tc_2",
		Scope: types.SpaceTagScope,
	}
	_, err = db.Core.NewInsert().Model(tc1).Exec(ctx)
	require.Empty(t, err)
	_, err = db.Core.NewInsert().Model(tc2).Exec(ctx)
	require.Empty(t, err)

	tags, err = ts.AllSpaceCategories(ctx)
	require.Empty(t, err)
	require.Len(t, tags, 1)
}

// TestCreateTag tests the CreateTag method
func TestTagStore_CreateTag(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	ts := database.NewTagStoreWithDB(db)
	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	t1, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, t1.ID)
}

// TestSetMetaTags tests the SetMetaTags method
func TestTagStore_SetMetaTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// insert a new repo
	rs := database.NewRepoStoreWithDB(db)
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("models_%s/%s", userName, repoName),
		Name:           repoName,
		Nickname:       "",
		Description:    "",
		Private:        false,
		RepositoryType: types.ModelRepo,
	})
	require.Empty(t, err)
	require.NotNil(t, repo)

	ts := database.NewTagStoreWithDB(db)
	var tags []*database.Tag
	_, err = ts.CreateCategory(ctx, database.TagCategory{
		Name: "industry",
	})
	require.Nil(t, err)
	_, err = ts.CreateCategory(ctx, database.TagCategory{Name: "task"})
	require.NoError(t, err)
	tag, err := ts.CreateTag(ctx, database.Tag{
		Name:     "xxx",
		Category: "industry",
		Scope:    types.ModelTagScope,
		BuiltIn:  true,
	})
	require.Nil(t, err)
	err = ts.UpsertRepoTags(ctx, repo.ID, []int64{}, []int64{tag.ID})
	require.Nil(t, err)

	getRepoTags, err := rs.Tags(ctx, repo.ID)
	require.Empty(t, err)
	require.Len(t, getRepoTags, 1)

	tags = append(tags, &database.Tag{
		Name:     "tag_" + uuid.NewString(),
		Category: "task",
		Group:    "",
		Scope:    types.ModelTagScope,
		BuiltIn:  true,
		ShowName: "",
	})
	tags = append(tags, &database.Tag{
		Name:     "tag_" + uuid.NewString(),
		Category: "framework",
		Group:    "",
		Scope:    types.ModelTagScope,
		BuiltIn:  true,
		ShowName: "",
	})
	for i, tag := range tags {
		updateTag, err := ts.CreateTag(ctx, *tag)
		require.NoError(t, err)
		tags[i] = updateTag
	}
	_, err = ts.SetMetaTags(ctx, types.ModelRepo, userName, repoName, tags)
	// should report err as framework tag is not allowed
	require.NotEmpty(t, err)

	tags = tags[:1]
	repoTags, err := ts.SetMetaTags(ctx, types.ModelRepo, userName, repoName, tags)
	require.Empty(t, err)
	require.Len(t, repoTags, 1)

	getRepoTags, err = rs.Tags(ctx, repo.ID)
	require.Empty(t, err)
	require.Len(t, getRepoTags, 2)
}

// TestSetLibraryTag tests the SetLibraryTag method
func TestTagStore_SetLibraryTag(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// insert a new repo
	rs := database.NewRepoStoreWithDB(db)
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("models_%s/%s", userName, repoName),
		Name:           repoName,
		Nickname:       "",
		Description:    "",
		Private:        false,
		RepositoryType: types.ModelRepo,
	})
	require.Empty(t, err)
	require.NotNil(t, repo)

	ts := database.NewTagStoreWithDB(db)
	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	oldTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	tag = database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	newTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	err = ts.SetLibraryTag(ctx, types.ModelRepo, userName, repoName, newTag, oldTag)
	require.Empty(t, err)

	repoTagStore := database.NewRepoStoreWithDB(db)
	tags, err := repoTagStore.Tags(ctx, repo.ID)
	require.Empty(t, err)
	require.Len(t, tags, 1)
}

// TestUpsertRepoTags tests the UpsertRepoTags method
func TestTagStore_UpsertRepoTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := database.NewTagStoreWithDB(db)
	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	oldTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)

	// insert a new repo with tags
	rs := database.NewRepoStoreWithDB(db)
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("models_%s/%s", userName, repoName),
		Name:           repoName,
		Nickname:       "",
		Description:    "",
		Private:        false,
		RepositoryType: types.ModelRepo,
		Tags:           []database.Tag{*oldTag},
	})
	require.Empty(t, err)
	require.NotNil(t, repo)
	require.Len(t, repo.Tags, 1)

	oldTagIds := make([]int64, 0, len(repo.Tags))
	oldTagIds = append(oldTagIds, repo.Tags[0].ID)

	tag = database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	newTag, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	newTagIds := make([]int64, 0, 2)
	newTagIds = append(newTagIds, newTag.ID)
	newTagIds = append(newTagIds, oldTag.ID)

	err = ts.UpsertRepoTags(ctx, repo.ID, oldTagIds, newTagIds)
	require.Empty(t, err)
	repoTags, err := rs.Tags(ctx, repo.ID)
	require.Empty(t, err)
	require.Len(t, repoTags, 2)
}

// TestRemoveRepoTags tests the RemoveRepoTags method
func TestTagStore_RemoveRepoTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := database.NewTagStoreWithDB(db)
	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	tag1, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, tag1.ID)

	tag = database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	tag2, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, tag2.ID)

	// insert a new repo with tags
	rs := database.NewRepoStoreWithDB(db)
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("models_%s/%s", userName, repoName),
		Name:           repoName,
		Nickname:       "",
		Description:    "",
		Private:        false,
		RepositoryType: types.ModelRepo,
	})
	require.Empty(t, err)
	require.NotNil(t, repo)
	// set repo tags
	err = ts.UpsertRepoTags(ctx, repo.ID, []int64{}, []int64{tag1.ID, tag2.ID})
	require.Empty(t, err)

	removeTagIds := make([]int64, 0, 1)
	removeTagIds = append(removeTagIds, tag2.ID)

	err = ts.RemoveRepoTags(ctx, repo.ID, removeTagIds)
	require.Empty(t, err)
	repoTags, err := rs.Tags(ctx, repo.ID)
	require.Empty(t, err)
	require.Len(t, repoTags, 1)
	require.EqualValues(t, repoTags[0], *tag1)

}

// TestRemoveRepoTags tests the RemoveRepoTagsByCategory method
func TestTagStore_RemoveRepoTagsByCategory(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := database.NewTagStoreWithDB(db)
	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	tag1, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, tag1.ID)
	tag = database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	tag2, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, tag2.ID)

	// insert a new repo with tags
	rs := database.NewRepoStoreWithDB(db)
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("models_%s/%s", userName, repoName),
		Name:           repoName,
		Nickname:       "",
		Description:    "",
		Private:        false,
		RepositoryType: types.ModelRepo,
	})
	require.Empty(t, err)
	require.NotNil(t, repo)
	// set repo tags
	err = ts.UpsertRepoTags(ctx, repo.ID, []int64{}, []int64{tag1.ID, tag2.ID})
	require.Empty(t, err)

	err = ts.RemoveRepoTagsByCategory(ctx, repo.ID, []string{"task"})
	require.Nil(t, err)

}

func TestTagStore_FindTagByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	ts := database.NewTagStoreWithDB(db)
	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	t1, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, t1.ID)

	tag1, err := ts.FindTagByID(ctx, t1.ID)
	require.Empty(t, err)
	require.Equal(t, tag1.ID, t1.ID)
	require.Equal(t, tag1.Name, t1.Name)
}

func TestTagStore_UpdateTagByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := database.NewTagStoreWithDB(db)

	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	t1, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, t1.ID)

	newName := "new_tag_" + uuid.NewString()

	t1.Name = newName

	tag1, err := ts.UpdateTagByID(ctx, t1)
	require.Empty(t, err)
	require.Equal(t, tag1.Name, newName)

}

func TestTagStore_DeleteTagByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := database.NewTagStoreWithDB(db)
	tag := database.Tag{
		Category: "task",
		Name:     "tag_" + uuid.New().String(),
		Group:    "",
		Scope:    types.ModelTagScope,
	}
	tag1, err := ts.CreateTag(ctx, tag)
	require.Empty(t, err)
	require.NotEmpty(t, tag1.ID)

	// insert a new repo with tags
	rs := database.NewRepoStoreWithDB(db)
	userName := "user_name_" + uuid.NewString()
	repoName := "repo_name_" + uuid.NewString()
	repo, err := rs.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           fmt.Sprintf("%s/%s", userName, repoName),
		GitPath:        fmt.Sprintf("models_%s/%s", userName, repoName),
		Name:           repoName,
		Nickname:       "",
		Description:    "",
		Private:        false,
		RepositoryType: types.ModelRepo,
	})
	require.Empty(t, err)
	require.NotNil(t, repo)

	// set repo tags
	err = ts.UpsertRepoTags(ctx, repo.ID, []int64{}, []int64{tag1.ID})
	require.Empty(t, err)

	err = ts.DeleteTagByID(ctx, tag1.ID)
	require.Empty(t, err)

	_, err = ts.FindTagByID(ctx, tag1.ID)
	require.NotEmpty(t, err)

}

func TestTagStore_Category_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := database.NewTagStoreWithDB(db)

	categories, err := ts.AllCategories(ctx, "")
	require.Empty(t, err)
	require.NotEmpty(t, categories)

	total := len(categories)

	catetory, err := ts.CreateCategory(ctx, database.TagCategory{
		Name:     "test-category",
		Scope:    "test-scope",
		ShowName: "测试分类",
		Enabled:  true,
	})
	require.Empty(t, err)
	require.NotEmpty(t, catetory)

	id := catetory.ID

	catetory, err = ts.UpdateCategory(ctx, database.TagCategory{
		ID:       1,
		Name:     "test-category1",
		Scope:    "test-scope1",
		ShowName: "测试分类1",
		Enabled:  false,
	})
	require.Empty(t, err)
	require.NotEmpty(t, catetory)

	categories, err = ts.AllCategories(ctx, "")
	require.Empty(t, err)
	require.NotEmpty(t, categories)
	require.Equal(t, "test-category1", categories[0].Name)
	require.Equal(t, "test-scope1", string(categories[0].Scope))
	require.Equal(t, "测试分类1", categories[0].ShowName)
	require.Equal(t, false, categories[0].Enabled)

	err = ts.DeleteCategory(ctx, id)
	require.Empty(t, err)

	categories, err = ts.AllCategories(ctx, "")
	require.Empty(t, err)
	require.Equal(t, total, len(categories))
}
