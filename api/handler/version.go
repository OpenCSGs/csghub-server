package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/version"
)

type VersionHandler struct {
}

func NewVersionHandler() *VersionHandler {
	return &VersionHandler{}
}

func (h *VersionHandler) Version(ctx *gin.Context) {
	httpbase.OK(ctx, gin.H{
		"version": version.StarhubAPIVersion,
		"commit":  version.GitRevision,
	})
}
