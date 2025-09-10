package router

import (
	"fmt"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"opencsg.com/csghub-server/accounting/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mq"
)

func NewAccountRouter(config *config.Config, mqHandler mq.MessageQueue) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log(config))
	needAPIKey := middleware.NeedAPIKey(config)
	//add router for golang pprof
	debugGroup := r.Group("/debug", needAPIKey)
	pprof.RouteRegister(debugGroup, "pprof")
	r.Use(middleware.Authenticator(config))
	err := createCustomValidator()
	if err != nil {
		return nil, fmt.Errorf("error create validator, error: %w", err)
	}
	apiGroup := r.Group("/api/v1/accounting")

	// metering
	meterHandler, err := handler.NewMeteringHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating multi sync handler, error: %w", err)
	}
	createMeteringRoutes(apiGroup, meterHandler)
	err = createAdvancedRoutes(apiGroup, config, mqHandler)
	if err != nil {
		return nil, fmt.Errorf("error creating accounting advanced routes, error:%w", err)
	}

	return r, nil
}

func createMeteringRoutes(apiGroup *gin.RouterGroup, meterHandler *handler.MeteringHandler) {
	meterGroup := apiGroup.Group("/metering")
	{
		meterGroup.GET("/:id/statements", meterHandler.QueryMeteringStatementByUserID)
		meterGroup.GET("/stat", meterHandler.QueryMeteringStatByDate)
	}
}

func createCustomValidator() error {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("optional_date_format", OptionalDateFormat); err != nil {
			return fmt.Errorf("fail to register custom validation functions:%w", err)
		}
		if err := v.RegisterValidation("year_month_format", OptionalYearMonthFormat); err != nil {
			return fmt.Errorf("fail to register custom validation functions:%w", err)
		}
		if err := v.RegisterValidation("phone_format", OptionalPhoneFormat); err != nil {
			return fmt.Errorf("fail to register custom validation functions:%w", err)
		}
		return nil // Return nil if no error occurred during registration of custom validation functions
	} else {
		return fmt.Errorf("fail to register custom validation functions")
	}
}
