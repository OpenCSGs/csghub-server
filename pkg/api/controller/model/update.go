package model

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/store/database"
	"opencsg.com/starhub-server/pkg/types"
	"opencsg.com/starhub-server/pkg/utils/common"
)

func (c *Controller) Update(ctx *gin.Context) (model *database.Model, err error) {
	var req types.UpdateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}

	_, err = c.namespaceStore.FindByPath(ctx, namespace)
	if err != nil {
		return nil, errors.New("Namespace does not exist")
	}

	_, err = c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("User does not exist")
	}

	model, err = c.modelStore.FindyByPath(ctx, namespace, name)
	if err != nil {
		return
	}

	err = c.gitServer.UpdateModelRepo(namespace, name, model, model.Repository, &req)
	if err == nil {
		err = c.modelStore.Update(ctx, model, model.Repository)
		if err != nil {
			return
		}
	}
	return
}
