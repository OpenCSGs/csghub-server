package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

type ImportHandler interface {
	Import(c *gin.Context)
	GetGitlabRepos(ctx *gin.Context)
	ImportStatus(ctx *gin.Context)
}

type importHandlerImpl struct {
	c component.ImportComponent
}

func NewImportHandler(config *config.Config) (ImportHandler, error) {
	c, err := component.NewImportComponent(config)
	if err != nil {
		return nil, err
	}
	return &importHandlerImpl{
		c: c,
	}, nil
}
