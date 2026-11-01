package component

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	commonutil "opencsg.com/csghub-server/common/utils/common"
)

type AgentSkillComponent interface {
	// ListSkills returns skills with agentichub-skills tag + user's own skills
	ListSkills(ctx context.Context, userUUID string, userName string, filter types.AgentSkillFilter, per, page int) ([]types.AgentSkillListItem, int, error)
}

func NewAgentSkillComponent(config *config.Config) (AgentSkillComponent, error) {
	c := &agentSkillComponentImpl{
		config:          config,
		agentSkillStore: database.NewAgentSkillStore(),
	}
	return c, nil
}

type agentSkillComponentImpl struct {
	config          *config.Config
	agentSkillStore database.AgentSkillStore
}

// ListSkills returns platform skills (with agentichub-skills tag) and user's own skills
func (c *agentSkillComponentImpl) ListSkills(ctx context.Context, userUUID string, userName string, filter types.AgentSkillFilter, per, page int) ([]types.AgentSkillListItem, int, error) {
	dbFilter := database.AgentSkillFilter{
		Search:  filter.Search,
		BuiltIn: filter.BuiltIn,
	}
	skills, total, err := c.agentSkillStore.ListForAgent(ctx, userUUID, userName, dbFilter, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get agent skills: %w", err)
	}
	for idx := range skills {
		if c.config == nil {
			continue
		}
		repo := &database.Repository{
			Path:           skills[idx].Path,
			RepositoryType: types.SkillRepo,
		}
		skills[idx].Repository = commonutil.BuildCloneInfo(c.config, repo)
		skills[idx].DownloadURL = buildAgentSkillDownloadURL(c.config.Model.DownloadEndpoint, skills[idx].Path, skills[idx].DefaultBranch)
	}
	return skills, total, nil
}

func buildAgentSkillDownloadURL(downloadEndpoint, repoPath, defaultBranch string) string {
	downloadEndpoint = strings.TrimSpace(downloadEndpoint)
	namespace, name, ok := strings.Cut(repoPath, "/")
	if downloadEndpoint == "" || !ok || namespace == "" || name == "" {
		return ""
	}
	ref := strings.TrimSpace(defaultBranch)
	if ref == "" {
		ref = types.MainBranch
	}
	downloadURL, err := url.JoinPath(downloadEndpoint, "api/v1", "skills", namespace, name, "download_archive", "refs", ref)
	if err != nil {
		return ""
	}
	return downloadURL
}
