package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

func NewTagHandler(config *config.Config) (*TagsHandler, error) {
	tc, err := component.NewTagComponent(config)
	if err != nil {
		return nil, err
	}
	return &TagsHandler{
		tc: tc,
	}, nil
}

type TagsHandler struct {
	tc *component.TagComponent
}

// GetAllTags godoc
// @Security     ApiKey
// @Summary      Get all tags
// @Description  get all tags
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.ResponseWithTotal{data=[]database.Tag,total=int} "tags"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags [get]
func (t *TagsHandler) AllTags(ctx *gin.Context) {
	//TODO:validate inputs

	tags, err := t.tc.AllTags(ctx)
	if err != nil {
		slog.Error("Failed to load tags", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data": tags,
	}

	slog.Info("Tags loaded successfully", "count", len(tags))
	ctx.JSON(http.StatusOK, respData)
}
