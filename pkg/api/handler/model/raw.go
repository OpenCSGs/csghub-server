package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/api/controller/model"
)

func HandleFileRaw(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		content, err := modelCtrl.FileRaw(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get model file content failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    content,
			"message": "Get model file content successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
