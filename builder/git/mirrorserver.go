package git

import (
	"errors"

	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver/gitea"
	"opencsg.com/csghub-server/common/config"
)

func NewMirrorServer(config *config.Config) (mirrorserver.MirrorServer, error) {
	if !config.MirrorServer.Enable && !config.Saas {
		return nil, nil
	}
	if config.MirrorServer.Type == "gitea" {
		mirrorServer, err := gitea.NewMirrorClient(config)
		return mirrorServer, err
	}

	return nil, errors.New("undefined mirror server type")
}
