package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/component"
	"opencsg.com/csghub-server/mirror/manager"
)

type ManagerHandler struct {
	manager          *manager.Manager
	managerComponent component.ManagerComponent
}

func NewManagerHandler(cfg *config.Config) (*ManagerHandler, error) {
	c, err := component.NewMirrorComponent(cfg)
	if err != nil {
		return nil, err
	}
	m, err := manager.GetManager(cfg)
	if err != nil {
		return nil, err
	}
	return &ManagerHandler{
		manager:          m,
		managerComponent: c,
	}, nil
}

type StopWorkerReq struct {
	Percent int `json:"percent"`
}

type StopWorkerByIDReq struct {
	ID int `json:"id"`
}

type SyncNowReq struct {
	MirrorID int64 `json:"mirror_id" binding:"required"`
	WorkerID int   `json:"worker_id"`
}

type CancelReq struct {
	MirrorID int64 `json:"mirror_id" binding:"required"`
}

func (h *ManagerHandler) StopWorkerByID(c *gin.Context) {
	var req StopWorkerByIDReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.manager.StopWorker(req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	httpbase.OK(c, nil)
}

func (h *ManagerHandler) SyncNow(c *gin.Context) {
	var req SyncNowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.managerComponent.SyncNow(c, req.WorkerID, req.MirrorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	httpbase.OK(c, nil)
}

func (h *ManagerHandler) Cancel(c *gin.Context) {
	var req CancelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	found, err := h.managerComponent.Cancel(c, req.MirrorID)
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
