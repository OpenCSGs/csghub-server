package component

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgitserver "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestClawHubComponent_SearchOnlyReturnsSkillsWithVersions(t *testing.T) {
	ctx := context.Background()
	skillComponent := mockcomponent.NewMockSkillComponent(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skill:             skillComponent,
		skillVersionStore: skillVersionStore,
		repoComponent:     mockcomponent.NewMockRepoComponent(t),
	}

	skillComponent.EXPECT().Index(ctx, &types.RepoFilter{Search: "agent", Username: "u"}, 10, 1, false, true).Return([]*types.Skill{
		{
			ID:          1,
			Name:        "published-skill-3-0-21",
			Nickname:    "published-skill-3-0-21",
			Path:        "u/published-skill-3-0-21",
			Description: "published",
			UpdatedAt:   time.Unix(0, 0),
		},
		{
			ID:          2,
			Name:        "bad-version-skill",
			Path:        "u/bad-version-skill",
			Description: "bad version",
		},
	}, 2, nil)
	skillVersionStore.EXPECT().LatestBySkillIDs(ctx, []int64{1, 2}).Return(map[int64]*database.SkillVersion{
		1: {
			ID:      11,
			SkillID: 1,
			Version: "v1.0.0",
		},
	}, nil)

	resp, err := component.Search(ctx, "agent", 10, "u")

	require.NoError(t, err)
	require.Equal(t, &types.ClawHubSearchResponse{
		Results: []types.ClawHubSearchResult{
			{
				Slug:        "u--published-skill",
				DisplayName: "published-skill",
				Summary:     "published",
				Version:     "1.0.0",
				Score:       1.0,
			},
		},
	}, resp)
}

func TestClawHubComponent_GetSkillUsesLatestForUnpublishedSkill(t *testing.T) {
	ctx := context.Background()
	skillComponent := mockcomponent.NewMockSkillComponent(t)
	component := &clawHubComponent{
		config:        &config.Config{},
		skill:         skillComponent,
		repoComponent: mockcomponent.NewMockRepoComponent(t),
	}

	createdAt := time.UnixMilli(1710000000000)
	updatedAt := time.UnixMilli(1710000001000)
	skillComponent.EXPECT().Show(ctx, "u", "draft-skill", "u", false, false).Return(&types.Skill{
		Name:        "draft-skill",
		Nickname:    "draft-skill",
		Description: "draft description",
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		User: types.User{
			Username: "u",
			Nickname: "U",
			Avatar:   "avatar.png",
		},
	}, nil)

	resp, err := component.GetSkill(ctx, "u--draft-skill-1-2-3", "u")

	require.NoError(t, err)
	require.Equal(t, "u--draft-skill", resp.Skill.Slug)
	require.Equal(t, "draft-skill", resp.Skill.DisplayName)
	require.Equal(t, "latest", resp.LatestVersion.Version)
	require.Len(t, resp.Versions, 1)
	require.Equal(t, "latest", resp.Versions[0].Version)
}

func TestClawHubComponent_GetSkillVersionReturnsSpecifiedVersion(t *testing.T) {
	ctx := context.Background()
	skillComponent := mockcomponent.NewMockSkillComponent(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skill:             skillComponent,
		skillVersionStore: skillVersionStore,
		repoComponent:     mockcomponent.NewMockRepoComponent(t),
	}

	createdAt := time.UnixMilli(1710000000000)
	updatedAt := time.UnixMilli(1710000001000)
	skillComponent.EXPECT().Show(ctx, "zhzhang", "auto-updater", "zhzhang", false, false).Return(&types.Skill{
		ID:          12,
		Name:        "auto-updater",
		Nickname:    "Auto Updater",
		Description: "updates things",
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		User: types.User{
			Username: "zhzhang",
			Nickname: "ZZ",
			Avatar:   "avatar.png",
		},
	}, nil)
	skillVersionStore.EXPECT().BySkillIDAndVersion(ctx, int64(12), "1.0.2").Return(&database.SkillVersion{
		SkillID:   12,
		Version:   "1.0.2",
		Hash:      "abc123",
		Changelog: "release",
	}, nil)

	resp, err := component.GetSkillVersion(ctx, "zhzhang--auto-updater", "1.0.2", "zhzhang")

	require.NoError(t, err)
	require.Equal(t, "zhzhang--auto-updater", resp.Skill.Slug)
	require.Equal(t, "Auto Updater", resp.Skill.DisplayName)
	require.Equal(t, "1.0.2", resp.Version.Version)
	require.Equal(t, "release", resp.Version.Changelog)
}

func TestClawHubComponent_GetSkillVersionFallsBackToVPrefixedVersion(t *testing.T) {
	ctx := context.Background()
	skillComponent := mockcomponent.NewMockSkillComponent(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skill:             skillComponent,
		skillVersionStore: skillVersionStore,
		repoComponent:     mockcomponent.NewMockRepoComponent(t),
	}

	skillComponent.EXPECT().Show(ctx, "zhzhang", "auto-updater", "zhzhang", false, false).Return(&types.Skill{
		ID:        12,
		Name:      "auto-updater",
		Nickname:  "Auto Updater",
		CreatedAt: time.UnixMilli(1710000000000),
		UpdatedAt: time.UnixMilli(1710000001000),
		User: types.User{
			Username: "zhzhang",
		},
	}, nil)
	skillVersionStore.EXPECT().BySkillIDAndVersion(ctx, int64(12), "1.0.2").Return(nil, sql.ErrNoRows)
	skillVersionStore.EXPECT().BySkillIDAndVersion(ctx, int64(12), "v1.0.2").Return(&database.SkillVersion{
		SkillID: 12,
		Version: "v1.0.2",
		Hash:    "abc123",
	}, nil)

	resp, err := component.GetSkillVersion(ctx, "zhzhang--auto-updater", "1.0.2", "zhzhang")

	require.NoError(t, err)
	require.Equal(t, "1.0.2", resp.Version.Version)
}

func TestClawHubComponent_PublishSkillCreatesSkillWhenNotFound(t *testing.T) {
	ctx := context.Background()
	skillComponent := mockcomponent.NewMockSkillComponent(t)
	skillStore := mockdatabase.NewMockSkillStore(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	userStore := mockdatabase.NewMockUserStore(t)
	gitServer := mockgitserver.NewMockGitServer(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skill:             skillComponent,
		skillStore:        skillStore,
		skillVersionStore: skillVersionStore,
		userStore:         userStore,
		gitServer:         gitServer,
		repoComponent:     mockcomponent.NewMockRepoComponent(t),
	}

	userStore.EXPECT().FindByUsername(ctx, "u").Return(database.User{Username: "u"}, nil)
	skillStore.EXPECT().FindByPath(ctx, "u", "draft-skill").Return(nil, sql.ErrNoRows).Once()
	skillComponent.EXPECT().Create(ctx, mock.MatchedBy(func(req *types.CreateSkillReq) bool {
		return req != nil && req.Namespace == "u" && req.Name == "draft-skill" && req.Username == "u"
	})).Return(&types.Skill{ID: 10}, nil)
	skillStore.EXPECT().FindByPath(ctx, "u", "draft-skill").Return(&database.Skill{ID: 10}, nil)
	gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: "u",
		Name:      "draft-skill",
		RepoType:  types.SkillRepo,
		Ref:       types.MainBranch,
	}).Return(nil, nil)
	skillVersionStore.EXPECT().BySkillIDAndVersion(ctx, int64(10), "v1.0.0").Return(nil, sql.ErrNoRows)
	skillVersionStore.EXPECT().Create(ctx, mock.MatchedBy(func(input database.SkillVersion) bool {
		return input.SkillID == 10 && input.Version == "v1.0.0"
	})).Return(&database.SkillVersion{ID: 20, SkillID: 10, Version: "v1.0.0"}, nil)

	resp, err := component.PublishSkill(ctx, &types.ClawHubPublishRequest{
		Slug:    "draft-skill",
		Version: "v1.0.0",
	}, map[string][]byte{
		"README.md": []byte("hello"),
	}, "u")

	require.NoError(t, err)
	require.True(t, resp.Ok)
	require.Equal(t, "10", resp.SkillId)
	require.Equal(t, "20", resp.VersionId)
}

func TestParseSkillNameAndVersion(t *testing.T) {
	name, version := parseSkillNameAndVersion("demo-skill-1.0.2")
	require.Equal(t, "demo-skill", name)
	require.Equal(t, "v1.0.2", version)

	name, version = parseSkillNameAndVersion("csghub-server-api")
	require.Equal(t, "csghub-server-api", name)
	require.Equal(t, "", version)

	name, version = parseSkillNameAndVersion("demo-skill-1-0-2")
	require.Equal(t, "demo-skill", name)
	require.Equal(t, "v1.0.2", version)

	name, version = parseSkillNameAndVersion("self-improving-agent-3-0-21")
	require.Equal(t, "self-improving-agent", name)
	require.Equal(t, "v3.0.21", version)

	name, version = parseSkillNameAndVersion("demo-skill-v1.0.2")
	require.Equal(t, "demo-skill-v1.0.2", name)
	require.Equal(t, "", version)
}

func TestClawHubComponent_PublishSkillUsesLatestWhenVersionNotSpecified(t *testing.T) {
	ctx := context.Background()
	skillComponent := mockcomponent.NewMockSkillComponent(t)
	skillStore := mockdatabase.NewMockSkillStore(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	userStore := mockdatabase.NewMockUserStore(t)
	gitServer := mockgitserver.NewMockGitServer(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skill:             skillComponent,
		skillStore:        skillStore,
		skillVersionStore: skillVersionStore,
		userStore:         userStore,
		gitServer:         gitServer,
		repoComponent:     mockcomponent.NewMockRepoComponent(t),
	}

	userStore.EXPECT().FindByUsername(ctx, "u").Return(database.User{Username: "u"}, nil)
	skillStore.EXPECT().FindByPath(ctx, "u", "demo-skill").Return(nil, sql.ErrNoRows).Once()
	skillComponent.EXPECT().Create(ctx, mock.MatchedBy(func(req *types.CreateSkillReq) bool {
		return req != nil &&
			req.Namespace == "u" &&
			req.Name == "demo-skill" &&
			req.Nickname == "demo-skill" &&
			req.Username == "u"
	})).Return(&types.Skill{ID: 11}, nil)
	skillStore.EXPECT().FindByPath(ctx, "u", "demo-skill").Return(&database.Skill{ID: 11}, nil)
	gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: "u",
		Name:      "demo-skill",
		RepoType:  types.SkillRepo,
		Ref:       types.MainBranch,
	}).Return(nil, nil)
	skillVersionStore.EXPECT().BySkillIDAndVersion(ctx, int64(11), "latest").Return(nil, sql.ErrNoRows)
	skillVersionStore.EXPECT().Create(ctx, mock.MatchedBy(func(input database.SkillVersion) bool {
		return input.SkillID == 11 && input.Version == "latest"
	})).Return(&database.SkillVersion{ID: 21, SkillID: 11, Version: "latest"}, nil)

	resp, err := component.PublishSkill(ctx, &types.ClawHubPublishRequest{
		Slug: "demo-skill-1-0-2",
	}, map[string][]byte{
		"README.md": []byte("hello"),
	}, "u")

	require.NoError(t, err)
	require.True(t, resp.Ok)
	require.Equal(t, "11", resp.SkillId)
	require.Equal(t, "21", resp.VersionId)
}

func TestResolveClawHubDisplayName(t *testing.T) {
	require.Equal(t, "x-search", resolveClawHubDisplayName("x-search-1.0.0", "", "x-search"))
	require.Equal(t, "x-search", resolveClawHubDisplayName("x-search-1.0.0", "x-search-1.0.0", "x-search"))
	require.Equal(t, "x-search", resolveClawHubDisplayName("x-search-1-0-0", "x-search-1-0-0", "x-search"))
	require.Equal(t, "Search Tool", resolveClawHubDisplayName("x-search-1.0.0", "Search Tool", "x-search"))
	require.Equal(t, "Proactive Agent", resolveClawHubDisplayName("proactive-agent-3-1-0", "Proactive Agent 3.1.0", "proactive-agent"))
	require.Equal(t, "Proactive Agent", resolveClawHubDisplayName("proactive-agent-3-1-0", "Proactive Agent v3.1.0", "proactive-agent"))
	require.Equal(t, "Proactive Agent", resolveClawHubDisplayName("proactive-agent-3-1-0", "Proactive Agent 3-1-0", "proactive-agent"))
}

func TestNormalizeClawHubSkillIdentity(t *testing.T) {
	slug, displayName := NormalizeClawHubSkillIdentity("self-improving-agent-3-0-21", "self-improving-agent-3-0-21")
	require.Equal(t, "self-improving-agent", slug)
	require.Equal(t, "self-improving-agent", displayName)

	slug, displayName = NormalizeClawHubSkillIdentity("self-improving-agent-3-0-21", "Self Improving Agent")
	require.Equal(t, "self-improving-agent", slug)
	require.Equal(t, "Self Improving Agent", displayName)

	slug, displayName = NormalizeClawHubSkillIdentity("proactive-agent-3-1-0", "Proactive Agent 3.1.0")
	require.Equal(t, "proactive-agent", slug)
	require.Equal(t, "Proactive Agent", displayName)
}

func TestClawHubComponent_PublishSkillReturnsFindError(t *testing.T) {
	ctx := context.Background()
	skillStore := mockdatabase.NewMockSkillStore(t)
	userStore := mockdatabase.NewMockUserStore(t)
	component := &clawHubComponent{
		config:        &config.Config{},
		skillStore:    skillStore,
		userStore:     userStore,
		repoComponent: mockcomponent.NewMockRepoComponent(t),
	}

	userStore.EXPECT().FindByUsername(ctx, "u").Return(database.User{Username: "u"}, nil)
	skillStore.EXPECT().FindByPath(ctx, "u", "broken-skill").Return(nil, errors.New("db down"))

	resp, err := component.PublishSkill(ctx, &types.ClawHubPublishRequest{
		Slug:    "broken-skill",
		Version: "v1.0.0",
	}, map[string][]byte{
		"README.md": []byte("hello"),
	}, "u")

	require.Nil(t, resp)
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to find skill")
	require.ErrorContains(t, err, "db down")
}

func TestClawHubComponent_PublishSkillForbiddenWithoutWritePermission(t *testing.T) {
	ctx := context.Background()
	skillStore := mockdatabase.NewMockSkillStore(t)
	userStore := mockdatabase.NewMockUserStore(t)
	repoComponent := mockcomponent.NewMockRepoComponent(t)
	component := &clawHubComponent{
		config:        &config.Config{},
		skillStore:    skillStore,
		userStore:     userStore,
		repoComponent: repoComponent,
	}

	repo := &database.Repository{ID: 1, Path: "owner/existing-skill", Name: "existing-skill", User: database.User{Username: "owner"}}
	userStore.EXPECT().FindByUsername(ctx, "attacker").Return(database.User{Username: "attacker"}, nil)
	skillStore.EXPECT().FindByPath(ctx, "attacker", "existing-skill").Return(&database.Skill{
		ID: 1, RepositoryID: 1, Repository: repo,
	}, nil)
	repoComponent.EXPECT().GetUserRepoPermission(ctx, "attacker", repo).Return(&types.UserRepoPermission{CanWrite: false}, nil)

	resp, err := component.PublishSkill(ctx, &types.ClawHubPublishRequest{
		Slug:    "existing-skill",
		Version: "v2.0.0",
	}, map[string][]byte{
		"README.md": []byte("malicious"),
	}, "attacker")

	require.Nil(t, resp)
	require.ErrorIs(t, err, errorx.ErrForbidden)
}

func TestClawHubComponent_DownloadSkillUsesLatestPublishedVersion(t *testing.T) {
	ctx := context.Background()
	skillStore := mockdatabase.NewMockSkillStore(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	gitServer := mockgitserver.NewMockGitServer(t)
	repoComponent := mockcomponent.NewMockRepoComponent(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skillStore:        skillStore,
		skillVersionStore: skillVersionStore,
		gitServer:         gitServer,
		repoComponent:     repoComponent,
	}

	repo := &database.Repository{Nickname: "test skill"}
	skillStore.EXPECT().FindByPath(ctx, "u", "test-skill").Return(&database.Skill{
		ID: 1, Repository: repo,
	}, nil)
	repoComponent.EXPECT().GetUserRepoPermission(ctx, "", repo).Return(&types.UserRepoPermission{CanRead: true}, nil)
	skillVersionStore.EXPECT().LatestBySkillID(ctx, int64(1)).Return(&database.SkillVersion{
		SkillID: 1,
		Version: "v1.2.3",
		Hash:    "commit-v123",
	}, nil)
	gitServer.EXPECT().GetArchive(ctx, gitserver.GetArchiveReq{
		Namespace: "u",
		Name:      "test-skill",
		Revision:  "commit-v123",
		RepoType:  types.SkillRepo,
	}).Return([]byte("zip-bytes"), nil)

	content, version, err := component.DownloadSkill(ctx, "u--test-skill-1-2-3", "latest", "")

	require.NoError(t, err)
	require.Equal(t, []byte("zip-bytes"), content)
	require.Equal(t, "1.2.3", version)
}

func TestClawHubComponent_ResolveSkillNormalizesSlug(t *testing.T) {
	ctx := context.Background()
	skillStore := mockdatabase.NewMockSkillStore(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	repoComponent := mockcomponent.NewMockRepoComponent(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skillStore:        skillStore,
		skillVersionStore: skillVersionStore,
		repoComponent:     repoComponent,
	}

	repo := &database.Repository{}
	skillStore.EXPECT().FindByPath(ctx, "u", "test-skill").Return(&database.Skill{ID: 1, Repository: repo}, nil)
	repoComponent.EXPECT().GetUserRepoPermission(ctx, "", repo).Return(&types.UserRepoPermission{CanRead: true}, nil)
	skillVersionStore.EXPECT().LatestBySkillID(ctx, int64(1)).Return(&database.SkillVersion{
		SkillID: 1,
		Version: "v1.2.3",
	}, nil)

	resp, err := component.ResolveSkill(ctx, "u--test-skill-1-2-3", "")

	require.NoError(t, err)
	require.Equal(t, "1.2.3", resp.LatestVersion.Version)
	require.Equal(t, "1.2.3", resp.Match.Version)
}

func TestClawHubComponent_DownloadSkillUsesLatestForUnpublishedSkill(t *testing.T) {
	ctx := context.Background()
	skillStore := mockdatabase.NewMockSkillStore(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	gitServer := mockgitserver.NewMockGitServer(t)
	repoComponent := mockcomponent.NewMockRepoComponent(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skillStore:        skillStore,
		skillVersionStore: skillVersionStore,
		gitServer:         gitServer,
		repoComponent:     repoComponent,
	}

	repo := &database.Repository{Nickname: "draft skill"}
	skillStore.EXPECT().FindByPath(ctx, "u", "draft-skill").Return(&database.Skill{
		ID: 2, Repository: repo,
	}, nil)
	repoComponent.EXPECT().GetUserRepoPermission(ctx, "", repo).Return(&types.UserRepoPermission{CanRead: true}, nil)
	skillVersionStore.EXPECT().LatestBySkillID(ctx, int64(2)).Return(nil, nil)
	gitServer.EXPECT().GetArchive(ctx, gitserver.GetArchiveReq{
		Namespace: "u",
		Name:      "draft-skill",
		Revision:  types.MainBranch,
		RepoType:  types.SkillRepo,
	}).Return([]byte("draft-zip"), nil)

	content, version, err := component.DownloadSkill(ctx, "u--draft-skill", "", "")

	require.NoError(t, err)
	require.Equal(t, []byte("draft-zip"), content)
	require.Equal(t, "latest", version)
}

func TestClawHubComponent_DownloadSkillUsesSpecifiedVersion(t *testing.T) {
	ctx := context.Background()
	skillStore := mockdatabase.NewMockSkillStore(t)
	skillVersionStore := mockdatabase.NewMockSkillVersionStore(t)
	gitServer := mockgitserver.NewMockGitServer(t)
	repoComponent := mockcomponent.NewMockRepoComponent(t)
	component := &clawHubComponent{
		config:            &config.Config{},
		skillStore:        skillStore,
		skillVersionStore: skillVersionStore,
		gitServer:         gitServer,
		repoComponent:     repoComponent,
	}

	repo := &database.Repository{}
	skillStore.EXPECT().FindByPath(ctx, "u", "versioned-skill").Return(&database.Skill{
		ID: 3, Repository: repo,
	}, nil)
	repoComponent.EXPECT().GetUserRepoPermission(ctx, "", repo).Return(&types.UserRepoPermission{CanRead: true}, nil)
	skillVersionStore.EXPECT().BySkillIDAndVersion(ctx, int64(3), "v2.0.0").Return(&database.SkillVersion{
		SkillID: 3,
		Version: "v2.0.0",
		Hash:    "commit-v200",
	}, nil)
	gitServer.EXPECT().GetArchive(ctx, gitserver.GetArchiveReq{
		Namespace: "u",
		Name:      "versioned-skill",
		Revision:  "commit-v200",
		RepoType:  types.SkillRepo,
	}).Return([]byte("versioned-zip"), nil)

	content, version, err := component.DownloadSkill(ctx, "u--versioned-skill", "v2.0.0", "")

	require.NoError(t, err)
	require.Equal(t, []byte("versioned-zip"), content)
	require.Equal(t, "2.0.0", version)
}

func TestBuildSkillSyncCommitFiles(t *testing.T) {
	commitFiles := buildSkillSyncCommitFiles(map[string][]byte{
		"README.md": []byte("new readme"),
		"skill.go":  []byte("package skill"),
	}, []*types.File{
		{Path: "README.md"},
	}, []*types.File{
		{Path: "README.md"},
		{Path: "old.txt"},
	})

	require.Equal(t, []gitserver.CommitFile{
		{
			Path:    "README.md",
			Content: base64.StdEncoding.EncodeToString([]byte("new readme")),
			Action:  gitserver.CommitActionUpdate,
		},
		{
			Path:    "skill.go",
			Content: base64.StdEncoding.EncodeToString([]byte("package skill")),
			Action:  gitserver.CommitActionCreate,
		},
		{
			Path:   "old.txt",
			Action: gitserver.CommitActionDelete,
		},
	}, commitFiles)
}

func TestClawHubComponent_CommitSkillFilesSyncsExistingRepo(t *testing.T) {
	ctx := context.Background()
	gitServer := mockgitserver.NewMockGitServer(t)
	component := &clawHubComponent{
		config:        &config.Config{},
		gitServer:     gitServer,
		repoComponent: mockcomponent.NewMockRepoComponent(t),
	}

	gitServer.EXPECT().GetFilesByRevisionAndPaths(ctx, gitserver.GetFilesByRevisionAndPathsReq{
		Namespace: "u",
		Name:      "test-skill",
		RepoType:  types.SkillRepo,
		Revision:  types.MainBranch,
		Paths:     []string{"README.md", "skill.go"},
	}).Return([]*types.File{
		{Path: "README.md"},
	}, nil)
	gitServer.EXPECT().GetRepoAllFiles(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: "u",
		Name:      "test-skill",
		RepoType:  types.SkillRepo,
		Ref:       types.MainBranch,
	}).Return([]*types.File{
		{Path: "README.md"},
		{Path: "old.txt"},
	}, nil)
	gitServer.EXPECT().CommitFiles(ctx, mock.MatchedBy(func(req gitserver.CommitFilesReq) bool {
		require.Equal(t, "u", req.Namespace)
		require.Equal(t, "test-skill", req.Name)
		require.Equal(t, types.SkillRepo, req.RepoType)
		require.Equal(t, types.MainBranch, req.Revision)
		require.Equal(t, "u", req.Username)
		require.Equal(t, "u@users.noreply.csghub.com", req.Email)
		require.Equal(t, "Publish version v1.0.0", req.Message)
		require.Equal(t, []gitserver.CommitFile{
			{
				Path:    "README.md",
				Content: base64.StdEncoding.EncodeToString([]byte("new readme")),
				Action:  gitserver.CommitActionUpdate,
			},
			{
				Path:    "skill.go",
				Content: base64.StdEncoding.EncodeToString([]byte("package skill")),
				Action:  gitserver.CommitActionCreate,
			},
			{
				Path:   "old.txt",
				Action: gitserver.CommitActionDelete,
			},
		}, req.Files)
		return true
	})).Return(nil)

	err := component.commitSkillFiles(ctx, map[string][]byte{
		"README.md": []byte("new readme"),
		"skill.go":  []byte("package skill"),
	}, "u", "u", "test-skill", "Publish version v1.0.0")

	require.NoError(t, err)
}

func TestClawHubComponent_CommitSkillFilesRetriesWithLatestTree(t *testing.T) {
	ctx := context.Background()
	gitServer := mockgitserver.NewMockGitServer(t)
	component := &clawHubComponent{
		config:        &config.Config{},
		gitServer:     gitServer,
		repoComponent: mockcomponent.NewMockRepoComponent(t),
	}

	gitServer.EXPECT().GetFilesByRevisionAndPaths(ctx, gitserver.GetFilesByRevisionAndPathsReq{
		Namespace: "u",
		Name:      "test-skill",
		RepoType:  types.SkillRepo,
		Revision:  types.MainBranch,
		Paths:     []string{"new.txt"},
	}).Return([]*types.File{}, nil).Once()
	gitServer.EXPECT().GetRepoAllFiles(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: "u",
		Name:      "test-skill",
		RepoType:  types.SkillRepo,
		Ref:       types.MainBranch,
	}).Return([]*types.File{}, nil).Once()
	gitServer.EXPECT().CommitFiles(ctx, mock.MatchedBy(func(req gitserver.CommitFilesReq) bool {
		return len(req.Files) == 1 && req.Files[0].Path == "new.txt" && req.Files[0].Action == gitserver.CommitActionCreate
	})).Return(errors.New("branch changed")).Once()

	gitServer.EXPECT().GetFilesByRevisionAndPaths(ctx, gitserver.GetFilesByRevisionAndPathsReq{
		Namespace: "u",
		Name:      "test-skill",
		RepoType:  types.SkillRepo,
		Revision:  types.MainBranch,
		Paths:     []string{"new.txt"},
	}).Return([]*types.File{
		{Path: "new.txt"},
	}, nil).Once()
	gitServer.EXPECT().GetRepoAllFiles(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: "u",
		Name:      "test-skill",
		RepoType:  types.SkillRepo,
		Ref:       types.MainBranch,
	}).Return([]*types.File{
		{Path: "new.txt"},
	}, nil).Once()
	gitServer.EXPECT().CommitFiles(ctx, mock.MatchedBy(func(req gitserver.CommitFilesReq) bool {
		return len(req.Files) == 1 && req.Files[0].Path == "new.txt" && req.Files[0].Action == gitserver.CommitActionUpdate
	})).Return(nil).Once()

	err := component.commitSkillFiles(ctx, map[string][]byte{
		"new.txt": []byte("content"),
	}, "u", "u", "test-skill", "Publish version v1.0.0")

	require.NoError(t, err)
}

func TestClawHubComponent_PublishSkillRejectsTooManyFiles(t *testing.T) {
	ctx := context.Background()
	skillStore := mockdatabase.NewMockSkillStore(t)
	userStore := mockdatabase.NewMockUserStore(t)
	component := &clawHubComponent{
		skillStore:    skillStore,
		userStore:     userStore,
		repoComponent: mockcomponent.NewMockRepoComponent(t),
		config:        &config.Config{},
	}
	component.config.Skill.MaxPublishFileCount = 2

	userStore.EXPECT().FindByUsername(ctx, "u").Return(database.User{Username: "u"}, nil)

	resp, err := component.PublishSkill(ctx, &types.ClawHubPublishRequest{
		Slug:    "test-skill",
		Version: "v1.0.0",
	}, map[string][]byte{
		"a.txt": []byte("a"),
		"b.txt": []byte("b"),
		"c.txt": []byte("c"),
	}, "u")

	require.Nil(t, resp)
	require.ErrorIs(t, err, errorx.ErrSkillPublishFileCountExceeded)
}

func TestClawHubComponent_PublishSkillRejectsOversizedFiles(t *testing.T) {
	ctx := context.Background()
	skillStore := mockdatabase.NewMockSkillStore(t)
	userStore := mockdatabase.NewMockUserStore(t)
	component := &clawHubComponent{
		skillStore:    skillStore,
		userStore:     userStore,
		repoComponent: mockcomponent.NewMockRepoComponent(t),
		config:        &config.Config{},
	}
	component.config.Skill.MaxPublishFileSize = 10

	userStore.EXPECT().FindByUsername(ctx, "u").Return(database.User{Username: "u"}, nil)

	resp, err := component.PublishSkill(ctx, &types.ClawHubPublishRequest{
		Slug:    "test-skill",
		Version: "v1.0.0",
	}, map[string][]byte{
		"a.txt": []byte("hello world"),
	}, "u")

	require.Nil(t, resp)
	require.ErrorIs(t, err, errorx.ErrSkillPublishFileSizeExceeded)
}
