package router

import (
	"fmt"
	"net/http"

	"opencsg.com/csghub-server/builder/instrumentation"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func NewRProxyRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	middleware.SetInfraMiddleware(r, config, instrumentation.RProxy)
	r.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowAllOrigins:  true,
	}))
	store := cookie.NewStore([]byte(config.Space.SessionSecretKey))
	store.Options(sessions.Options{
		// SameSite: http.SameSiteNoneMode, // support 3rd part
		SameSite: http.SameSiteLaxMode,
		Secure:   config.EnableHTTPS,
		HttpOnly: true,
	})
	r.Use(sessions.Sessions("opencsg_jwt_session", store))
	//to access space with jwt token in query string
	r.Use(middleware.BuildJwtSession(config.JWT.SigningKey))
	//to access model,fintune with any kind of tokens in auth header
	r.Use(middleware.Authenticator(config))

	handler, err := handler.NewRProxyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating rproxy handler:%w", err)
	}
	r.Any("/*api", middleware.AuthSession(), handler.Proxy)

	return r, nil
}
