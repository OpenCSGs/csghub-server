package model

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleDetail(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		modelDetail, err := modelCtrl.Detail(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": "Request failed.",
			})
			return
		}

		respData := gin.H{
			"code":   200,
			"detail": modelDetail,
		}

		c.JSON(http.StatusOK, respData)
	}
}
