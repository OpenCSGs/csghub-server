package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/model"
)

func HandleDelete(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := modelCtrl.Delete(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Delete model failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Model deleted.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
