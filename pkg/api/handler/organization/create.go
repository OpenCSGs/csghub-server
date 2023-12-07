package organization

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/organization"
	"github.com/gin-gonic/gin"
)

func HandleCreate(orgCtrl *organization.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		org, err := orgCtrl.Create(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Created organization failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Organization created.",
			"data":    org,
		}

		c.JSON(http.StatusOK, respData)
	}
}
