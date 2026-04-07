package component

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	mock_rpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mock_database "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSkillComponent_Create(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestSkillComponent(ctx, t)

	req := &types.CreateSkillReq{
		CreateRepoReq: types.CreateRepoReq{
			Username:  "user",
			Namespace: "ns",
			Name:      "n",
			License:   "l",
			Readme:    "r",
		},
	}
	dbrepo := &database.Repository{
		ID:   1,
		Path: "ns/n",
		User: database.User{Username: "user", UUID: "user-uuid"},
		Tags: []database.Tag{{Name: "t1"}},
	}
	crq := req.CreateRepoReq
	crq.Nickname = "n"
	crq.Readme = generateReadmeData(req.License)
	crq.RepoType = types.SkillRepo
	crq.DefaultBranch = "main"
	crq.CommitFiles = []types.CommitFile{
		{
			Content: crq.Readme,
			Path:    types.ReadmeFileName,
		},
		{
			Content: skillGitattributesContent,
			Path:    types.GitattributesFileName,
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	cc.mocks.components.repo.EXPECT().
		SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
			return req.RepoType == types.SkillRepo &&
				req.Operation == types.OperationCreate &&
				req.RepoPath == "ns/n" &&
				req.UserUUID == "user-uuid"
		})).
		RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
			wg.Done()
			return nil
		}).Once()
	cc.mocks.components.repo.EXPECT().CreateRepo(ctx, crq).Return(
		nil, dbrepo, &gitserver.CommitFilesReq{}, nil,
	)

	cc.mocks.gitServer.EXPECT().CommitFiles(ctx, gitserver.CommitFilesReq{}).Return(nil)
	cc.mocks.stores.SkillMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.Skill{
		Repository:   dbrepo,
		RepositoryID: 1,
	}, "ns/n").Return(&database.Skill{
		RepositoryID: 1,
		Repository:   dbrepo,
	}, nil)

	resp, err := cc.Create(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.Skill{
		RepositoryID: 1,
		User: types.User{
			Username: "user",
		},
		Path: "ns/n",
		Repository: types.Repository{
			HTTPCloneURL: "/s/ns/n.git",
			SSHCloneURL:  ":s/ns/n.git",
		},
		Tags: []types.RepoTag{{Name: "t1"}},
	}, resp)
	wg.Wait()
}

func TestSkillComponent_Index(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestSkillComponent(ctx, t)

	filter := &types.RepoFilter{Username: "user"}
	repos := []*database.Repository{
		{ID: 1, Name: "r1", Tags: []database.Tag{{Name: "t1"}}},
		{ID: 2, Name: "r2"},
		{ID: 5, Name: "r2"},
	}
	cc.mocks.components.repo.EXPECT().PublicToUser(ctx, types.SkillRepo, "user", filter, 10, 1).Return(
		repos, 100, nil,
	)
	cc.mocks.stores.SkillMock().EXPECT().ByRepoIDs(ctx, []int64{1, 2, 5}).Return([]database.Skill{
		{ID: 11, RepositoryID: 2, Repository: &database.Repository{ID: 2, Name: "r2", Mirror: database.Mirror{}}},
		{ID: 12, RepositoryID: 1, Repository: &database.Repository{ID: 2, Name: "r2", Mirror: database.Mirror{}}},
		{ID: 13, RepositoryID: 6},
	}, nil)

	data, total, err := cc.Index(ctx, filter, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []*types.Skill{
		{ID: 12, RepositoryID: 1, Name: "r1", Tags: []types.RepoTag{{Name: "t1"}}, RecomOpWeight: 0},
		{ID: 11, RepositoryID: 2, Name: "r2", RecomOpWeight: 0},
	}, data)
}

func TestSkillComponent_Index_HalfCreatedRepos(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestSkillComponent(ctx, t)

	filter := &types.RepoFilter{Username: "user"}
	// PublicToUser returns 3 repositories, but only 2 have corresponding skills
	cc.mocks.components.repo.EXPECT().PublicToUser(ctx, types.SkillRepo, "user", filter, 10, 1).Return(
		[]*database.Repository{
			{ID: 1, Name: "r1", Tags: []database.Tag{{Name: "t1"}}},
			{ID: 2, Name: "r2"},
			{ID: 3, Name: "half-created", Tags: []database.Tag{{Name: "t3"}}}, // This is a half-created repo with no skill
		}, 3, nil, // Total should be 3
	)

	// ByRepoIDs returns only 2 skills (no skill for repo ID 3)
	cc.mocks.stores.SkillMock().EXPECT().ByRepoIDs(ctx, []int64{1, 2, 3}).Return([]database.Skill{
		{ID: 11, RepositoryID: 2, Repository: &database.Repository{ID: 2, Name: "r2", Mirror: database.Mirror{}}},
		{ID: 12, RepositoryID: 1, Repository: &database.Repository{ID: 1, Name: "r1", Mirror: database.Mirror{}}},
	}, nil)

	data, total, err := cc.Index(ctx, filter, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 3, total) // Total should match PublicToUser's return value
	require.Len(t, data, 2)    // But only 2 skills should be returned

	require.Equal(t, []*types.Skill{
		{ID: 12, RepositoryID: 1, Name: "r1", Tags: []types.RepoTag{{Name: "t1"}}, RecomOpWeight: 0},
		{ID: 11, RepositoryID: 2, Name: "r2", RecomOpWeight: 0},
	}, data)
}

func TestSkillComponent_Update(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestSkillComponent(ctx, t)

	req := &types.UpdateSkillReq{
		UpdateRepoReq: types.UpdateRepoReq{
			RepoType: types.SkillRepo,
		},
	}
	dbrepo := &database.Repository{Name: "name"}
	cc.mocks.components.repo.EXPECT().UpdateRepo(ctx, req.UpdateRepoReq).Return(dbrepo, nil)
	cc.mocks.stores.SkillMock().EXPECT().ByRepoID(ctx, dbrepo.ID).Return(&database.Skill{ID: 1}, nil)
	cc.mocks.stores.SkillMock().EXPECT().Update(ctx, database.Skill{
		ID: 1,
	}).Return(nil)

	data, err := cc.Update(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.Skill{ID: 1, Name: "name"}, data)

}

func TestSkillComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestSkillComponent(ctx, t)

	skill := &database.Skill{
		ID:           123,
		RepositoryID: 1,
		Repository: &database.Repository{
			ID: 1,
			User: database.User{
				UUID: "owner-uuid",
			},
			Path: "skill-path",
		},
	}
	repo := &database.Repository{
		ID: 1,
		User: database.User{
			UUID: "owner-uuid",
		},
		Path: "ns/n",
	}
	cc.mocks.stores.SkillMock().EXPECT().FindByPath(ctx, "ns", "n").Return(skill, nil)
	cc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.SkillRepo,
	}).Return(repo, nil)

	cc.mocks.stores.SkillMock().EXPECT().Delete(ctx, *skill).Return(nil)
	var wg sync.WaitGroup
	wg.Add(1)
	cc.mocks.components.repo.EXPECT().
		SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
			return req.RepoType == types.SkillRepo &&
				req.Operation == types.OperationDelete &&
				req.RepoPath == "ns/n" &&
				req.UserUUID == "owner-uuid"
		})).
		RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
			wg.Done()
			return nil
		}).Once()
	err := cc.Delete(ctx, "ns", "n", "user")
	require.Nil(t, err)
	wg.Wait()
}

func TestSkillComponent_Show(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestSkillComponent(ctx, t)

	skill := &database.Skill{ID: 1, Repository: &database.Repository{
		ID: 11, Name: "name", User: database.User{Username: "user"}, SyncStatus: types.SyncStatusInProgress,
	}}
	cc.mocks.stores.SkillMock().EXPECT().FindByPath(ctx, "ns", "n").Return(skill, nil)
	cc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", skill.Repository).Return(
		&types.UserRepoPermission{CanRead: true, CanAdmin: true}, nil,
	)
	cc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", int64(11)).Return(true, nil)
	cc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil)

	cc.mocks.components.repo.EXPECT().GetMirrorTaskStatus(skill.Repository).Return(
		types.MirrorRepoSyncStart,
	)
	data, err := cc.Show(ctx, "ns", "n", "user", false, false)
	require.Nil(t, err)
	require.Equal(t, &types.Skill{
		ID: 1,
		Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		},
		RepositoryID:         11,
		Namespace:            &types.Namespace{},
		Name:                 "name",
		User:                 types.User{Username: "user"},
		CanManage:            true,
		UserLikes:            true,
		SensitiveCheckStatus: "Pending",
		MirrorTaskStatus:     types.MirrorRepoSyncStart,
		SyncStatus:           types.SyncStatusInProgress,
	}, data)
}

func TestSkillComponent_Relations(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestSkillComponent(ctx, t)

	cc.mocks.stores.SkillMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Skill{
		Repository:   &database.Repository{},
		RepositoryID: 1,
	}, nil)
	cc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, &database.Repository{}, "user").Return(true, nil)
	cc.mocks.components.repo.EXPECT().RelatedRepos(ctx, int64(1), "user").Return(
		map[types.RepositoryType][]*database.Repository{
			types.ModelRepo: {
				{Name: "r1"},
			},
		}, nil,
	)

	data, err := cc.Relations(ctx, "ns", "n", "user")
	require.Nil(t, err)
	require.Equal(t, &types.Relations{
		Models: []*types.Model{{Name: "r1"}},
	}, data)

}

func TestSkillComponent_OrgSkills(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestSkillComponent(ctx, t)

	cc.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, "ns", "user").Return(membership.RoleAdmin, nil)
	cc.mocks.stores.SkillMock().EXPECT().ByOrgPath(ctx, "ns", 10, 1, false).Return(
		[]database.Skill{{
			ID: 1, Repository: &database.Repository{Name: "repo"},
			RepositoryID: 11,
		}}, 100, nil,
	)

	data, total, err := cc.OrgSkills(ctx, &types.OrgSkillsReq{
		Namespace: "ns", CurrentUser: "user",
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Skill{
		{ID: 1, Name: "repo", RepositoryID: 11},
	}, data)

}

// New tests for OrgSkills with different scenarios
func TestSkillComponent_OrgSkills_OnlyPublic(t *testing.T) {
	// Setup mock stores
	mockSkillStore := new(mock_database.MockSkillStore)
	mockUserSvcClient := new(mock_rpc.MockUserSvcClient)
	
	// Mock response
	expectedSkills := []database.Skill{
		{
			ID:           1,
			RepositoryID: 123,
			Repository: &database.Repository{
				Name:          "test-skill",
				Nickname:      "Test Skill",
				Description:   "Test skill description",
				Likes:         10,
				DownloadCount: 5,
				Path:          "test-org/test-skill",
				Private:       false,
			},
			LastUpdatedAt: time.Now(),
		},
	}
	expectedTotal := 1
	
	// Setup mock expectations - user is not a member
	mockUserSvcClient.On("GetMemberRole", mock.Anything, "test-org", "test-user").Return(membership.RoleUnknown, nil)
	mockSkillStore.On("ByOrgPath", mock.Anything, "test-org", 10, 1, true).Return(expectedSkills, expectedTotal, nil)
	
	// Create skill component with mock dependencies
	component := &skillComponentImpl{
		skillStore:     mockSkillStore,
		userSvcClient:  mockUserSvcClient,
	}
	
	// Create request
	req := &types.OrgSkillsReq{
		Namespace:   "test-org",
		CurrentUser: "test-user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	
	// Call method
	skills, total, err := component.OrgSkills(context.Background(), req)
	
	// Assert results
	assert.NoError(t, err)
	assert.Len(t, skills, 1)
	assert.Equal(t, expectedTotal, total)
	assert.Equal(t, "test-skill", skills[0].Name)
	assert.Equal(t, "Test Skill", skills[0].Nickname)
	assert.Equal(t, "Test skill description", skills[0].Description)
	assert.Equal(t, int64(10), skills[0].Likes)
	assert.Equal(t, int64(5), skills[0].Downloads)
	assert.Equal(t, "test-org/test-skill", skills[0].Path)
	assert.Equal(t, int64(123), skills[0].RepositoryID)
	assert.False(t, skills[0].Private)
	
	// Verify mocks
	mockUserSvcClient.AssertCalled(t, "GetMemberRole", mock.Anything, "test-org", "test-user")
	mockSkillStore.AssertCalled(t, "ByOrgPath", mock.Anything, "test-org", 10, 1, true)
}

func TestSkillComponent_OrgSkills_Error(t *testing.T) {
	// Setup mock stores
	mockSkillStore := new(mock_database.MockSkillStore)
	mockUserSvcClient := new(mock_rpc.MockUserSvcClient)
	
	// Setup mock expectations
	mockUserSvcClient.On("GetMemberRole", mock.Anything, "test-org", "test-user").Return(membership.RoleAdmin, nil)
	mockSkillStore.On("ByOrgPath", mock.Anything, "test-org", 10, 1, false).Return(nil, 0, assert.AnError)
	
	// Create skill component with mock dependencies
	component := &skillComponentImpl{
		skillStore:     mockSkillStore,
		userSvcClient:  mockUserSvcClient,
	}
	
	// Create request
	req := &types.OrgSkillsReq{
		Namespace:   "test-org",
		CurrentUser: "test-user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	
	// Call method
	skills, total, err := component.OrgSkills(context.Background(), req)
	
	// Assert results
	assert.Error(t, err)
	assert.Nil(t, skills)
	assert.Equal(t, 0, total)
	
	// Verify mocks
	mockUserSvcClient.AssertCalled(t, "GetMemberRole", mock.Anything, "test-org", "test-user")
	mockSkillStore.AssertCalled(t, "ByOrgPath", mock.Anything, "test-org", 10, 1, false)
}
