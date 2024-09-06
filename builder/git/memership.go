package git

import (
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/git/membership/gitea"
	"opencsg.com/csghub-server/common/config"
)

func NewMemberShip(config config.Config) (membership.GitMemerShip, error) {
	c, err := gitea.NewClient(&config)
	return c, err
}
