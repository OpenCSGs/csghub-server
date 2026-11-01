//go:build !ee && !saas

package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

func handleRoutingInputPlanError(c *gin.Context, p *types.RequestPlan, err error) bool { return false }

func (h *OpenAIHandlerImpl) retainAutoRoutingBody(string) bool { return false }
