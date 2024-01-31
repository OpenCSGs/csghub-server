package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewSpaceHandler(config *config.Config) (*SpaceHandler, error) {
	tc, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, err
	}
	return &SpaceHandler{
		c: tc,
	}, nil
}

type SpaceHandler struct {
	c *component.SpaceComponent
}

func (h *SpaceHandler) Index(ctx *gin.Context) {
}

func (h *SpaceHandler) Get(ctx *gin.Context) {
}

// CreateSpace   godoc
// @Security     ApiKey
// @Summary      Create a new space
// @Description  create a new space
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        body body types.CreateSpaceReq true "body"
// @Success      200  {object}  types.Response{data=database.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces [post]
func (h *SpaceHandler) Create(ctx *gin.Context) {
	var req types.CreateSpaceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	space, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create space", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create space succeed", slog.String("space", space.Name))
	httpbase.OK(ctx, space)
}

func (h *SpaceHandler) Update(ctx *gin.Context) {
	type updateSpaceReq struct {
		Username string `json:"username" example:"creator_user_name"`
		License  string `json:"license" example:"MIT"`
		Private  bool   `json:"private"`
	}
}

func (h *SpaceHandler) Delete(ctx *gin.Context) {
}

func (h *SpaceHandler) Predict(ctx *gin.Context) {
}
