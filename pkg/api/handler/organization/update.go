package organization

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/api/controller/organization"
)

func HandleUpdate(orgCtrl *organization.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		org, err := orgCtrl.Update(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Updated failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Organization updated.",
			"data":    org,
		}

		c.JSON(http.StatusOK, respData)
	}
}