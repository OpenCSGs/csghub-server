package member

import (
	"fmt"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
)

type Controller struct {
	memberStore *database.MemberStore
	orgStore    *database.OrgStore
	gitServer   gitserver.GitServer
}

func New(config *config.Config) (*Controller, error) {
	gs, err := gitserver.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitserver:%w", err)
	}
	return &Controller{
		memberStore: database.NewMemberStore(),
		orgStore:    database.NewOrgStore(),
		gitServer:   gs,
	}, nil
}
