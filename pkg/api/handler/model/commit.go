package model

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleCommits(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		commits, err := modelCtrl.Commits(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": "Get model commits failed.",
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"commits": commits,
		}

		c.JSON(http.StatusOK, respData)
	}
}
