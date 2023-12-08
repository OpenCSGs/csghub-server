package gitserver

import "opencsg.com/starhub-server/config"

func ProvideGitServer(config *config.Config) (GitServer, error) {
	return NewGitServer(config)
}
