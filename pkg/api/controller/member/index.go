package member

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/store/database"
)

func (c *Controller) Index(ctx *gin.Context) (members []database.Member, err error) {
	return
}
