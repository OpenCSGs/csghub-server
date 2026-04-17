package checker

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSkillFileChecker_Check_NonSkillRepo(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "models/foo/bar", // Not a skill repo
	})

	require.Nil(t, err)
	require.True(t, valid)
}

func TestSkillFileChecker_Check_ValidExistingSkillFile(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists and has required metadata in the new content
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "",
		GitAlternateObjectDirectoriesRelative: nil,
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("---\nname: Test Skill\ndescription: This is a test skill\n---", nil)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
	})

	require.Nil(t, err)
	require.True(t, valid)
}

func TestSkillFileChecker_Check_InvalidExistingSkillFile_ValidNewSkillFile(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists and has required metadata in the new content
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("---\nname: Test Skill\ndescription: This is a test skill\n---", nil)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.Nil(t, err)
	require.True(t, valid)
}

func TestSkillFileChecker_Check_InvalidNewSkillFile(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists and has required metadata in the new content
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("Invalid SKILL.md", nil)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.NotNil(t, err)
	require.False(t, valid)
}

func TestSkillFileChecker_Check_NoSkillFile(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists and has required metadata in the new content
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("", fmt.Errorf("file not found"))

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.NotNil(t, err)
	require.False(t, valid)
}

func TestSkillFileChecker_Check_SkillFileWithExtraFields(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists and has required metadata with extra fields
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("---\nname: Test Skill\ndescription: This is a test skill\nauthor: Test Author\nversion: 1.0.0\ntags: [test, skill]\n---", nil)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.Nil(t, err)
	require.True(t, valid)
}

func TestSkillFileChecker_Check_SkillFileMissingName(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists but missing name field
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("---\ndescription: This is a test skill\n---", nil)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.NotNil(t, err)
	require.False(t, valid)
}

func TestSkillFileChecker_Check_SkillFileMissingDescription(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists but missing description field
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("---\nname: Test Skill\n---", nil)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.NotNil(t, err)
	require.False(t, valid)
}

func TestSkillFileChecker_Check_SkillFileInvalidYAML(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists but has invalid YAML
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("---\nname: Test Skill\ndescription: This is a test skill\ninvalid: : : :\n---", nil)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.NotNil(t, err)
	require.False(t, valid)
}

func TestSkillFileChecker_Check_SkillFileDifferentOrder(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestSkillFileChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.SkillRepo, "foo", "bar").
		Return(repo, nil)

	// Check if SKILL.md exists with fields in different order
	c.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             "foo",
		Name:                                  "bar",
		RepoType:                              types.SkillRepo,
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
		Path:                                  "SKILL.md",
		Ref:                                   "main",
	}).Return("---\ndescription: This is a test skill\nname: Test Skill\n---", nil)

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "skills/foo/bar",
		Changes:      "abc main",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.Nil(t, err)
	require.True(t, valid)
}
