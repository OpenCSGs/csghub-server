package git

import (
	"errors"

	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/git/membership/gitea"
	"opencsg.com/csghub-server/common/config"
)

func NewMemberShip(config config.Config) (membership.GitMemerShip, error) {
	if config.GitServer.Type == "gitea" {
		c, err := gitea.NewClient(&config)
		return c, err
	}
	return nil, errors.New("undefined git server type")
}
