//go:build !saas && !ee

package component

import (
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
)

func NewGitHTTPComponentImpl(config *config.Config) (GitHTTPComponent, error) {
	c := &gitHTTPComponentImpl{}
	c.config = config
	var err error
	c.gitServer, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.s3Client, err = s3.NewMinio(config)
	if err != nil {
		newError := fmt.Errorf("fail to init s3 client for code,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.s3Core, err = s3.NewMinioCore(config)
	if err != nil {
		return nil, err
	}
	c.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	c.repoStore = database.NewRepoStore()
	c.lfsLockStore = database.NewLfsLockStore()
	c.userStore = database.NewUserStore()
	c.mirrorStore = database.NewMirrorStore()
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}
