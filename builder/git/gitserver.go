package git

import (
	"errors"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitea"
	"opencsg.com/csghub-server/common/config"
)

func NewGitServer(config *config.Config) (gitserver.GitServer, error) {
	if config.GitServer.Type == "gitea" {
		gitServer, err := gitea.NewClient(config)
		return gitServer, err
	}

	return nil, errors.New("undefined git server type")
}
