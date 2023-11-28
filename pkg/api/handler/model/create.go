package model

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleCreate(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		model, err := modelCtrl.Create(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Created failed. %v", err),
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
