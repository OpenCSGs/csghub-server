package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/api/controller/model"
)

func HandleCommits(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		commits, err := modelCtrl.Commits(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get model commits failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    commits,
			"message": "Get model commits successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
