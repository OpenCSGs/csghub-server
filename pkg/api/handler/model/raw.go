package model

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleFileRaw(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		content, err := modelCtrl.FileRaw(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": "Get model file content failed.",
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"content": content,
		}

		c.JSON(http.StatusOK, respData)
	}
}
