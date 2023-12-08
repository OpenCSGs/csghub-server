package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/api/controller/model"
)

func HandleCreate(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		model, err := modelCtrl.Create(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Created model failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Model created.",
			"data":    model,
		}

		c.JSON(http.StatusOK, respData)
	}
}
