package router

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/moderation/handler"
)

func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log(config))

	needAPIKey := middleware.NeedAPIKey(config)

	//add router for golang pprof
	debugGroup := r.Group("/debug", needAPIKey)
	pprof.RouteRegister(debugGroup, "pprof")

	// r.Use(middleware.Authenticator(config))

	apiV1Group := r.Group("/api/v1")
	{
		apiV1Group.GET("/healthz", healthz)
	}

	mc, err := handler.NewRepoHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating repo handler:%w", err)
	}
	apiV1Group.POST("/repo", mc.FullCheck)
	sc, err := handler.NewSensitiveHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive handler:%w", err)
	}
	apiV1Group.POST("/text", sc.Text)
	apiV1Group.POST("/image", sc.Image)
	apiV1Group.POST("/llmresp", sc.LlmResp)
	apiV1Group.POST("/llmprompt", sc.LlmPrompt)

	adminGroup := apiV1Group.Group("/admin")

	// route: /admin/*
	createAdminRoutes(adminGroup)

	return r, nil
}

func healthz(ctx *gin.Context) {
	ctx.Writer.WriteHeader(http.StatusOK)
}
