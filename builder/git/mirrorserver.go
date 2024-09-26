package git

import (
	"errors"

	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver/gitea"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewMirrorServer(config *config.Config) (mirrorserver.MirrorServer, error) {
	if !config.MirrorServer.Enable {
		return nil, nil
	}
	if config.MirrorServer.Type == types.GitServerTypeGitea {
		mirrorServer, err := gitea.NewMirrorClient(config)
		return mirrorServer, err
	}

	return nil, errors.New("undefined mirror server type")
}
