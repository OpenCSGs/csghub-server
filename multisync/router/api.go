package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/multisync/handler"
)

func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log())
	// store := cookie.NewStore([]byte(config.Mirror.SessionSecretKey))
	// store.Options(sessions.Options{
	// SameSite: http.SameSiteNoneMode,
	// Secure:   config.EnableHTTPS,
	// })
	// r.Use(sessions.Sessions("jwt_session", store))
	// r.Use(middleware.BuildJwtSession(config.JWT.SigningKey))

	mpHandler, err := handler.NewMirrorProxyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating rproxy handler:%w", err)
	}
	rGroup := r.Group("/:repo_type/:namespace/:name")
	{
		rGroup.POST("/git-upload-pack", mpHandler.Serve)
		rGroup.POST("/git-receive-pack", mpHandler.Serve)
		rGroup.GET("/info/refs", mpHandler.Serve)
		rGroup.GET("/HEAD", mpHandler.Serve)
		rGroup.GET("/objects/info/alternates", mpHandler.Serve)
		rGroup.GET("/objects/info/http-alternates", mpHandler.Serve)
		rGroup.GET("/objects/info/packs", mpHandler.Serve)
		rGroup.GET("/objects/info/:file", mpHandler.Serve)
		rGroup.GET("/objects/:head/:hash", mpHandler.Serve)
		rGroup.GET("/objects/pack/pack-:file", mpHandler.Serve)
		rGroup.POST("/info/lfs/objects/batch", mpHandler.ServeLFS)
		rGroup.GET("/info/lfs/objects/:oid", mpHandler.ServeLFS)
	}

	// r.Any("/*api", handler.Serve)

	return r, nil
}
