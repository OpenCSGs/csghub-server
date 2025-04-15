//go:build !saas

package component

import (
	"opencsg.com/csghub-server/builder/store/database"
)

func (c *repoComponentImpl) allowPublic(repo *database.Repository) (allow bool, reason string) {
	//always allow public repo in on-premises deployment
	return true, ""
}
