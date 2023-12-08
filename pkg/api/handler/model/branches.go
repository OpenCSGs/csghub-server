package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/api/controller/model"
)

func HandleBranches(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validations
		branches, err := modelCtrl.Branches(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get model branches failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    branches,
			"message": "Get model branches successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
