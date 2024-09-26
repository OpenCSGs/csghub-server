package git

import (
	"errors"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/git/gitserver/gitea"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewGitServer(config *config.Config) (gitserver.GitServer, error) {
	if config.GitServer.Type == types.GitServerTypeGitea {
		gitServer, err := gitea.NewClient(config)
		return gitServer, err
	} else if config.GitServer.Type == types.GitServerTypeGitaly {
		gitServer, err := gitaly.NewClient(config)
		return gitServer, err
	}

	return nil, errors.New("undefined git server type")
}
