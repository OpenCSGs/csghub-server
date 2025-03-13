package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log(config))
	//to access model,fintune with any kind of tokens in auth header
	r.Use(middleware.Authenticator(config))
	mustLogin := middleware.MustLogin()

	handler, err := handler.NewOpenAIHandlerFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating openai handler from config:%w", err)
	}
	r.GET("/v1/models", mustLogin, handler.ListModels)
	r.GET("/v1/models/:model", mustLogin, handler.GetModel)
	r.POST("/v1/chat/completions", mustLogin, handler.Chat)

	return r, nil
}
