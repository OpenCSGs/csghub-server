package git

import (
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/common/config"
)

func NewGitServer(config *config.Config) (gitserver.GitServer, error) {
	gitServer, err := gitaly.NewClient(config)
	return gitServer, err
}
