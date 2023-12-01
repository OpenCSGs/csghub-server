package model

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleDelete(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := modelCtrl.Delete(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Delete model repository failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Model repository deleted.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
