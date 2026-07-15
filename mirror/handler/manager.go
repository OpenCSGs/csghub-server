package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/component"
)

type ManagerHandler struct {
	managerComponent component.ManagerComponent
}

func NewManagerHandler(cfg *config.Config) (*ManagerHandler, error) {
	c, err := component.NewMirrorComponent(cfg)
	if err != nil {
		return nil, err
	}
	return &ManagerHandler{
		managerComponent: c,
	}, nil
}

type CancelReq struct {
	TaskID int64 `json:"task_id" binding:"required"`
}

func (h *ManagerHandler) StopWorkerByID(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": "LFS worker manager has been replaced by workhub jobs"})
}

func (h *ManagerHandler) SyncNow(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": "manual LFS worker rerun has been replaced by workhub jobs"})
}

func (h *ManagerHandler) Cancel(c *gin.Context) {
	var req CancelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	found, err := h.managerComponent.Cancel(c, req.TaskID)
	if err != nil {
		if found {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
	}
	httpbase.OK(c, nil)
}

func (h *ManagerHandler) ListTasks(c *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(c)
	if err != nil {
		httpbase.BadRequestWithExt(c, err)
		return
	}
	tasks, err := h.managerComponent.ListTasks(c.Request.Context(), per, page)
	if err != nil {
		httpbase.ServerError(c, err)
		return
	}
	httpbase.OK(c, tasks)
}
