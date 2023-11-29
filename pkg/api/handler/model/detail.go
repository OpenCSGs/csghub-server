package model

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleDetail(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		modelDetail, err := modelCtrl.Detail(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Get model detail failed: %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    modelDetail,
			"message": "Get model detail successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
