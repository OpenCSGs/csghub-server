package model

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleCreate(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		model, err := modelCtrl.Create(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": "Created failed.",
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Dataset repository created.",
			"model":   model,
		}

		c.JSON(http.StatusOK, respData)
	}
}
