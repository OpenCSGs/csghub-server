package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/model"
)

func HandleFileCreate(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := modelCtrl.FileCreate(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Create model file failed: %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Create model file successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}

func HandleFileUpdate(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := modelCtrl.FileUpdate(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Update model file failed: %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Update model file successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
