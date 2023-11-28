package model

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleUpdate(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		model, err := modelCtrl.Update(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": "Updated failed.",
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Dataset repository updated.",
			"model":   model,
		}

		c.JSON(http.StatusOK, respData)
	}
}
