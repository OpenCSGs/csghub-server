package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type AgentSkillHandler struct {
	skill component.AgentSkillComponent
}

func NewAgentSkillHandler(config *config.Config) (*AgentSkillHandler, error) {
	skillComp, err := component.NewAgentSkillComponent(config)
	if err != nil {
		return nil, err
	}
	return &AgentSkillHandler{
		skill: skillComp,
	}, nil
}

// ListSkills godoc
// @Security     ApiKey
// @Summary      List agent skills (platform + user-created)
// @Description  list agent skills including platform skills with 'agentichub-skills' tag and user-created skills
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        built_in query bool false "filter: true=platform skills only, false=user-created only"
// @Param        per query int false "per" default(20)
// @Param        page query int false "page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.AgentSkillListItem,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/skills [get]
func (h *AgentSkillHandler) ListSkills(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	currentUser := httpbase.GetCurrentUser(ctx)

	filter := types.AgentSkillFilter{
		Search: ctx.Query("search"),
	}

	// Parse built_in filter
	if builtInStr := ctx.Query("built_in"); builtInStr != "" {
		builtIn, err := strconv.ParseBool(builtInStr)
		if err != nil {
			httpbase.BadRequest(ctx, "built_in must be true or false")
			return
		}
		filter.BuiltIn = &builtIn
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	skills, total, err := h.skill.ListSkills(ctx.Request.Context(), currentUserUUID, currentUser, filter, per, page)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to list agent skills", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, skills, total)
}
