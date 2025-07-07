package middleware

import (
	"database/sql"
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func RepoExists(cfg *config.Config) gin.HandlerFunc {
	repoComp, err := component.NewRepoComponent(cfg)
	return func(c *gin.Context) {
		if err != nil {
			httpbase.ServerError(c, err)
			c.Abort()
			return
		}
		namespace := c.Param("namespace")
		name := c.Param("name")
		// If namespace and name are empty, it means that the route is not a repo route, so we can skip the check
		if namespace != "" && name != "" {
			_, err := repoComp.IsExists(c, common.RepoTypeFromContext(c), namespace, name)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					httpbase.NotFoundError(c, err)
					c.Abort()
					return
				}
				httpbase.ServerError(c, err)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
