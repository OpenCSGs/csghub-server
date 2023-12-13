package model

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
)

func (c *Controller) FileCreate(ctx *gin.Context) (err error) {
	var req *types.CreateFileReq
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	if err = ctx.ShouldBindJSON(&req); err != nil {
		return err
	}
	filePath := ctx.Param("file_path")
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath

	_, err = c.namespaceStore.FindByPath(ctx, namespace)
	if err != nil {
		return errors.New("Namespace does not exist")
	}

	_, err = c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return errors.New("User does not exist")
	}
	err = c.gitServer.CreateModelFile(req)
	if err != nil {
		return
	}
	return
}

func (c *Controller) FileUpdate(ctx *gin.Context) (err error) {
	var req *types.UpdateFileReq
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	if err = ctx.ShouldBindJSON(&req); err != nil {
		return err
	}
	filePath := ctx.Param("file_path")

	_, err = c.namespaceStore.FindByPath(ctx, namespace)
	if err != nil {
		return errors.New("Namespace does not exist")
	}

	_, err = c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return errors.New("User does not exist")
	}

	err = c.gitServer.UpdateModelFile(namespace, name, filePath, req)
	if err != nil {
		return
	}
	return
}
