package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/api/controller/model"
)

func HandleIndex(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		models, total, err := modelCtrl.Index(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get model list failed. %v", err),
			})
			return
		}
		respData := gin.H{
			"code":        200,
			"total_count": total,
			"data":        models,
			"message":     "Get model list successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
