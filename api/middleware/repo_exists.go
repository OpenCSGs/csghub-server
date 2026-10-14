package middleware

import (
	"fmt"

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
			exists, err := repoComp.IsExists(c, common.RepoTypeFromContext(c), namespace, name)
			if err != nil {
				httpbase.ServerError(c, err)
				c.Abort()
				return
			}
			if !exists {
				httpbase.NotFoundError(c, fmt.Errorf("repository %s/%s not found", namespace, name))
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
