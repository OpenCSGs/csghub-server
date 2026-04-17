package checker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type SkillFileChecker struct {
	repoStore database.RepoStore
	gitServer gitserver.GitServer
	config    *config.Config
}

func NewSkillFileChecker(config *config.Config) (GitCallbackChecker, error) {
	git, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server: %w", err)
	}
	return &SkillFileChecker{
		repoStore: database.NewRepoStore(),
		gitServer: git,
		config:    config,
	}, nil
}

func (c *SkillFileChecker) Check(ctx context.Context, req types.GitalyAllowedReq) (bool, error) {
	var ref string
	repoType, namespace, name := req.GetRepoTypeNamespaceAndName()

	// Only check skill repositories
	if repoType != types.SkillRepo {
		return true, nil
	}

	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, err: %v", err)
	}
	if repo == nil {
		return false, errors.New("repo not found")
	}
	changes := strings.Split(req.Changes, " ")
	if len(changes) > 1 {
		ref = changes[1]
	}

	// Check if SKILL.md exists and has required metadata in the new content
	skillsContent, err := c.gitServer.GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace:                             namespace,
		Name:                                  name,
		RepoType:                              repoType,
		GitObjectDirectoryRelative:            req.GitEnv.GitObjectDirectoryRelative,
		GitAlternateObjectDirectoriesRelative: req.GitEnv.GitAlternateObjectDirectoriesRelative,
		Path:                                  "SKILL.md",
		Ref:                                   ref,
	})
	if err != nil {
		// SKILL.md not found in new content, reject push
		return false, fmt.Errorf("skill repository must have a SKILL.md file with name and description metadata")
	}

	// Check if SKILL.md is in the correct YAML format with name and description
	// Extract YAML front matter
	yamlContent := skillsContent
	if strings.HasPrefix(yamlContent, "---") {
		parts := strings.SplitN(yamlContent, "---", 3)
		if len(parts) >= 2 {
			yamlContent = parts[1]
		}
	}

	// Parse YAML content
	var metadata map[string]interface{}
	err = yaml.Unmarshal([]byte(yamlContent), &metadata)
	if err != nil {
		return false, fmt.Errorf("SKILL.md must be in valid YAML format: %w", err)
	}

	// Check if required fields are present
	if _, ok := metadata["name"]; !ok {
		return false, fmt.Errorf("SKILL.md must have a 'name' field")
	}
	if _, ok := metadata["description"]; !ok {
		return false, fmt.Errorf("SKILL.md must have a 'description' field")
	}

	return true, nil
}
