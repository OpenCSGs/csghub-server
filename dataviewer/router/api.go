package router

import (
	"fmt"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/dataviewer/handler"
	"opencsg.com/csghub-server/dataviewer/workflows"
)

type DataViewerService struct {
	viewerHandler   *handler.DatasetViewerHandler
	callbackHandler *handler.CallbackHandler
	gitServer       gitserver.GitServer
}

func NewDataViewerRouter(config *config.Config, tc temporal.Client) (*gin.Engine, error) {
	r := gin.New()
	if config.Instrumentation.OTLPEndpoint != "" {
		r.Use(otelgin.Middleware("csghub-dataviewer"))
	}
	r.Use(gin.Recovery())
	r.Use(middleware.Log())
	needAPIKey := middleware.NeedAPIKey(config)

	//add router for golang pprof
	debugGroup := r.Group("/debug", needAPIKey)
	pprof.RouteRegister(debugGroup, "pprof")

	r.Use(middleware.Authenticator(config))
	apiGroup := r.Group("/api/v1")
	datasetsGrp := apiGroup.Group("/datasets/:namespace/:name")

	dvService, err := createHandlers(config, tc)
	if err != nil {
		return nil, fmt.Errorf("creating handlers error: %w", err)
	}

	activity, err := workflows.NewDataViewerActivity(config, dvService.gitServer)
	if err != nil {
		return nil, fmt.Errorf("failed to create new data viewer activity: %w", err)
	}

	err = workflows.BuildDataViewerRunWorker(tc, config, activity)
	if err != nil {
		return nil, fmt.Errorf("building dataviewer run workers error: %w", err)
	}

	createDataViewerRoutes(datasetsGrp, dvService.viewerHandler)
	createCallbackRoutes(datasetsGrp, dvService.callbackHandler)

	return r, nil
}

func createHandlers(cfg *config.Config, tc temporal.Client) (
	dataViewerService *DataViewerService,
	err error,
) {
	dataViewerService = &DataViewerService{}
	gs, err := git.NewGitServer(cfg)
	if err != nil {
		err = fmt.Errorf("failed to create git server cause: %w", err)
		return
	}
	dataViewerService.gitServer = gs
	dataViewerService.viewerHandler, err = handler.NewDatasetViewerHandler(cfg, gs)
	if err != nil {
		err = fmt.Errorf("creating dataset viewer handler: %w", err)
		return
	}

	dataViewerService.callbackHandler, err = handler.NewCallBackHandler(cfg, tc, gs)
	if err != nil {
		err = fmt.Errorf("creating viewer callback handler: %w", err)
		return
	}

	return
}

func createDataViewerRoutes(datasetsGrp *gin.RouterGroup, dsViewerHandler *handler.DatasetViewerHandler) {
	fileViewerGrp := datasetsGrp.Group("/viewer")
	{
		fileViewerGrp.GET("/*file_path", dsViewerHandler.View)
	}
	dataViewerGrp := datasetsGrp.Group("/dataviewer")
	{
		dataViewerGrp.GET("/catalog", dsViewerHandler.Catalog)
		dataViewerGrp.GET("/rows", dsViewerHandler.Rows)
	}
}

func createCallbackRoutes(datasetsGrp *gin.RouterGroup, dsCallbackHandler *handler.CallbackHandler) {
	callbackGrp := datasetsGrp.Group("/callback")
	{
		callbackGrp.POST("/:branch", dsCallbackHandler.Callback)
	}
}
