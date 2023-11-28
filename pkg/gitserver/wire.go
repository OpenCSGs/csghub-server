package gitserver

import "git-devops.opencsg.com/product/community/starhub-server/config"

func ProvideGitServer(config *config.Config) (GitServer, error) {
	return NewGitServer(config)
}
