package organization

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/organization"
	"github.com/gin-gonic/gin"
)

func HandleDelete(orgCtrl *organization.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := orgCtrl.Delete(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Delete org repository failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Organization deleted.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
