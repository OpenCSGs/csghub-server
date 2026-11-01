//go:build !ee && !saas

package filter

import "opencsg.com/csghub-server/common/config"

func NewRepoFilter(config *config.Config) RepoFilter {
	return nil
}
