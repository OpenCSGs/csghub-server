package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

// AgentHandler handles agent-related API requests
type AgentHandler struct {
	agent component.AgentComponent
}

// NewAgentHandler creates a new AgentHandler
func NewAgentHandler(config *config.Config) (*AgentHandler, error) {
	agentComp, err := component.NewAgentComponent(config)
	if err != nil {
		return nil, err
	}

	return &AgentHandler{
		agent: agentComp,
	}, nil
}

// CreateTemplate godoc
// @Security     ApiKey
// @Summary      Create a new agent template
// @Description  Create a new agent template
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        body body types.AgentTemplate true "Agent template data"
// @Success      200  {object}  types.Response{data=types.AgentTemplate} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/templates [post]
func (h *AgentHandler) CreateTemplate(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	var template types.AgentTemplate
	if err := ctx.ShouldBindJSON(&template); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if template.Content == nil {
		httpbase.BadRequest(ctx, "content is required when creating a new template")
		return
	}

	// Set the current user as the owner
	template.UserUUID = &currentUserUUID

	err := h.agent.CreateTemplate(ctx.Request.Context(), &template)
	if err != nil {
		slog.Error("Failed to create agent template", "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, template)
}

// GetTemplate godoc
// @Security     ApiKey
// @Summary      Get an agent template by ID
// @Description  Get details of a specific agent template
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Template ID"
// @Success      200  {object}  types.Response{data=types.AgentTemplate} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/templates/{id} [get]
func (h *AgentHandler) GetTemplate(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "Invalid template ID")
		return
	}

	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	template, err := h.agent.GetTemplateByID(ctx.Request.Context(), id, currentUserUUID)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to get agent template", "id", id, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to get agent template", "id", id, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, template)
}

// ListTemplates godoc
// @Security     ApiKey
// @Summary      List agent templates for the current user
// @Description  Get all agent templates belonging to the current user
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        search query string false "search text"
// @Param        type query string false "type" Enums(langflow, code)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.AgentTemplate,total=int} "OK"
// @Failure      401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/templates [get]
func (h *AgentHandler) ListTemplates(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	//TODO: pagination
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var filter types.AgentTemplateFilter
	filter.Search = ctx.Query("search")
	filter.Type = ctx.Query("type")

	templates, total, err := h.agent.ListTemplatesByUserUUID(ctx.Request.Context(), currentUserUUID, filter, per, page)
	if err != nil {
		slog.Error("Failed to list agent templates", "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, templates, total)
}

// UpdateTemplate godoc
// @Security     ApiKey
// @Summary      Update an existing agent template
// @Description  Update the details of an existing agent template
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Template ID"
// @Param        body body types.AgentTemplate true "Updated template data"
// @Success      200  {object}  types.Response{data=types.AgentTemplate} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/templates/{id} [put]
func (h *AgentHandler) UpdateTemplate(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "Invalid template ID")
		return
	}

	var template types.AgentTemplate
	if err := ctx.ShouldBindJSON(&template); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	// Ensure the ID in the URL matches the ID in the request body
	template.ID = id

	// Set the current user as the owner (to ensure ownership check in component)
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	template.UserUUID = &currentUserUUID

	if err := h.agent.UpdateTemplate(ctx.Request.Context(), &template); err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to update agent template", "id", id, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to update agent template", "id", id, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, template)
}

// DeleteTemplate godoc
// @Security     ApiKey
// @Summary      Delete an agent template
// @Description  Permanently delete an agent template
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Template ID"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/templates/{id} [delete]
func (h *AgentHandler) DeleteTemplate(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "Invalid template ID")
		return
	}

	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	if err := h.agent.DeleteTemplate(ctx.Request.Context(), id, currentUserUUID); err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to delete agent template", "id", id, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to delete agent template", "id", id, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// CreateInstance godoc
// @Security     ApiKey
// @Summary      Create a new agent instance
// @Description  Create a new agent instance from a template
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        body body types.AgentInstance true "Agent instance data"
// @Success      200  {object}  types.Response{data=types.AgentInstance} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances [post]
func (h *AgentHandler) CreateInstance(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	var instance types.AgentInstance
	if err := ctx.ShouldBindJSON(&instance); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if instance.Type == nil {
		httpbase.BadRequest(ctx, "type is required")
		return
	}

	if instance.Name == nil {
		httpbase.BadRequest(ctx, "name is required")
		return
	}

	public := false
	if instance.Public == nil {
		instance.Public = &public
	}

	// Set the current user as the owner
	instance.UserUUID = &currentUserUUID

	err := h.agent.CreateInstance(ctx.Request.Context(), &instance)
	if err != nil {
		//check for forbidden error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to create agent instance under template", "template_id", instance.TemplateID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to create agent instance", "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, instance)
}

// GetInstance godoc
// @Security     ApiKey
// @Summary      Get an agent instance by ID
// @Description  Get details of a specific agent instance
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Instance ID"
// @Success      200  {object}  types.Response{data=types.AgentInstance} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id} [get]
func (h *AgentHandler) GetInstance(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "Invalid instance ID")
		return
	}

	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	instance, err := h.agent.GetInstanceByID(ctx.Request.Context(), id, currentUserUUID)
	if err != nil {
		//check for forbidden error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to get agent instance", "id", id, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to get agent instance", "id", id, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, instance)
}

// ListInstances godoc
// @Security     ApiKey
// @Summary      List agent instances for the current user
// @Description  Get all agent instances belonging to the current user
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        search query string false "search text"
// @Param        type query string false "type" Enums(langflow, code)
// @Param        built_in query bool false "built in" default(false)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.AgentInstance,total=int} "OK"
// @Failure      401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances [get]
func (h *AgentHandler) ListInstances(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var filter types.AgentInstanceFilter
	filter.Search = ctx.Query("search")
	filter.Type = ctx.Query("type")
	builtInStr := ctx.Query("built_in")
	if builtInStr != "" {
		builtIn, err := strconv.ParseBool(builtInStr)
		if err != nil {
			httpbase.BadRequest(ctx, "Invalid built_in value")
			return
		}
		filter.BuiltIn = &builtIn
	}

	instances, total, err := h.agent.ListInstancesByUserUUID(ctx.Request.Context(), currentUserUUID, filter, per, page)
	if err != nil {
		slog.Error("Failed to list agent instance by user uuid", "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, instances, total)
}

// ListInstancesByTemplate godoc
// @Security     ApiKey
// @Summary      List agent instances by template ID
// @Description  Get all agent instances created from a specific template
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        template_id path int64 true "Template ID"
// @Param        search query string false "search term"
// @Param        type query string false "filter by type"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.AgentInstance,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/templates/{id}/instances [get]
func (h *AgentHandler) ListInstancesByTemplate(ctx *gin.Context) {
	// Verify user is authenticated
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	templateIDStr := ctx.Param("id")
	templateID, err := strconv.ParseInt(templateIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "Invalid template ID")
		return
	}

	// Extract pagination parameters
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	// Create filter with template ID
	var filter types.AgentInstanceFilter
	filter.Search = ctx.Query("search")
	filter.Type = ctx.Query("type")
	filter.TemplateID = &templateID

	instances, total, err := h.agent.ListInstancesByUserUUID(ctx.Request.Context(), currentUserUUID, filter, per, page)
	if err != nil {
		slog.Error("Failed to list agent instances by template", "template_id", templateID, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, instances, total)
}

// UpdateInstance godoc
// @Security     ApiKey
// @Summary      Update an existing agent instance
// @Description  Update the details of an existing agent instance
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Instance ID"
// @Param        body body types.AgentInstance true "Updated instance data"
// @Success      200  {object}  types.Response{data=nil} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id} [put]
func (h *AgentHandler) UpdateInstance(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "Invalid instance ID")
		return
	}

	var instance types.AgentInstance
	if err := ctx.ShouldBindJSON(&instance); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	// Ensure the ID in the URL matches the ID in the request body
	instance.ID = id

	// Set the current user as the owner (to ensure ownership check in component)
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	instance.UserUUID = &currentUserUUID

	if err := h.agent.UpdateInstance(ctx.Request.Context(), &instance); err != nil {
		//check for permission error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to update agent instance", "id", id, "user_uuid", currentUserUUID, "instance_id", instance.ID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to update agent instance", "id", id, "user_uuid", currentUserUUID, "instance_id", instance.ID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// update instance by type and content id
// @Security     ApiKey
// @Summary      Update an existing agent instance by type and content id
// @Description  Update the details of an existing agent instance by type and content id
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        type path string true "type"
// @Param        content_id path string true "content id"
// @Param        body body types.UpdateAgentInstanceRequest true "Updated instance data"
// @Success      200  {object}  types.Response{data=types.AgentInstance} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/by-content-id/{type}/{content_id} [put]
func (h *AgentHandler) UpdateInstanceByContentID(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	instanceType := ctx.Param("type")
	instanceContentID := ctx.Param("content_id")
	instanceContentID = strings.TrimPrefix(instanceContentID, "/")
	if instanceContentID == "" {
		httpbase.BadRequest(ctx, "Invalid content id")
		return
	}

	var updateRequest types.UpdateAgentInstanceRequest
	if err := ctx.ShouldBindJSON(&updateRequest); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	instance, err := h.agent.UpdateInstanceByContentID(ctx.Request.Context(), currentUserUUID, instanceType, instanceContentID, updateRequest)
	if err != nil {
		//check for permission error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to update agent instance by type and content id", "type", instanceType, "content_id", instanceContentID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to update agent instance by type and content id", "type", instanceType, "content_id", instanceContentID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, instance)
}

// DeleteInstance godoc
// @Security     ApiKey
// @Summary      Delete an agent instance
// @Description  Permanently delete an agent instance
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Instance ID"
// @Success      200  {object}  types.Response{data=nil} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id} [delete]
func (h *AgentHandler) DeleteInstance(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "Invalid instance ID")
		return
	}

	userUUID := httpbase.GetCurrentUserUUID(ctx)
	if err := h.agent.DeleteInstance(ctx.Request.Context(), id, userUUID); err != nil {
		//check for permission error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to delete agent instance", "id", id, "user_uuid", userUUID, "instance_id", id)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to delete agent instance", "id", id, "user_uuid", userUUID, "instance_id", id, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// delete instance by type and content id
// @Security     ApiKey
// @Summary      Delete an agent instance by type and content id
// @Description  Permanently delete an agent instance by type and content id
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        type path string true "type"
// @Param        content_id path string true "content id"
// @Success      200  {object}  types.Response{data=nil} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/by-content-id/{type}/{content_id} [delete]
func (h *AgentHandler) DeleteInstanceByContentID(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	instanceType := ctx.Param("type")
	instanceContentID := ctx.Param("content_id")
	instanceContentID = strings.TrimPrefix(instanceContentID, "/")
	if instanceContentID == "" {
		httpbase.BadRequest(ctx, "Invalid content id")
		return
	}

	if err := h.agent.DeleteInstanceByContentID(ctx.Request.Context(), currentUserUUID, instanceType, instanceContentID); err != nil {
		//check for permission error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to delete agent instance by type and content id", "type", instanceType, "content_id", instanceContentID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to delete agent instance by type and content id", "type", instanceType, "content_id", instanceContentID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// ListSessions godoc
// @Security     ApiKey
// @Summary      List sessions by instance ID
// @Description  List all sessions for a specific agent instance with pagination
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        per query int false "page size"
// @Param        page query int false "current page number"
// @Param        search query string false "search by session name"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.AgentInstanceSession,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions [get]
func (h *AgentHandler) ListSessions(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	// Extract pagination parameters
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var filter types.AgentInstanceSessionFilter
	filter.InstanceID = &instanceID
	filter.Search = ctx.Query("search")

	sessions, total, err := h.agent.ListSessions(ctx.Request.Context(), currentUserUUID, filter, per, page)
	if err != nil {
		slog.Error("Failed to list chat sessions by instance id", "instance_id", filter.InstanceID, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, sessions, total)
}

// CreateSession creates a new session for an agent instance
// @Security     ApiKey
// @Summary      Create a new session for an agent instance
// @Description  Create a new session for an agent instance
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        body body types.CreateAgentInstanceSessionRequest true "Session data"
// @Success      200  {object}  types.Response{data=types.CreateAgentInstanceSessionResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions [post]
func (h *AgentHandler) CreateSession(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	var req types.CreateAgentInstanceSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	// Set the instance ID from URL parameter
	req.InstanceID = &instanceID

	sessionUUID, err := h.agent.CreateSession(ctx.Request.Context(), currentUserUUID, &req)
	if err != nil {
		slog.Error("Failed to create session", "user_uuid", currentUserUUID, "instance_id", instanceID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, types.CreateAgentInstanceSessionResponse{
		SessionUUID: sessionUUID,
	})
}

// get session by session uuid
// @Security     ApiKey
// @Summary      Get session by session UUID
// @Description  Get a session by session UUID
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        session_uuid path string true "Session UUID"
// @Success      200  {object}  types.Response{data=types.AgentInstanceSession} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions/{session_uuid} [get]
func (h *AgentHandler) GetSession(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	sessionUUID := ctx.Param("session_uuid")
	if sessionUUID == "" {
		httpbase.BadRequest(ctx, "Invalid session UUID")
		return
	}

	session, err := h.agent.GetSessionByUUID(ctx.Request.Context(), currentUserUUID, sessionUUID, instanceID)
	if err != nil {
		//check for permission error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to get session by session uuid", "session_uuid", sessionUUID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to get session by session uuid", "session_uuid", sessionUUID, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, session)
}

// delete session by session uuid
// @Security     ApiKey
// @Summary      Delete session by session UUID
// @Description  Delete a session by session UUID
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        session_uuid path string true "Session UUID"
// @Success      200  {object}  types.Response{data=nil} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions/{session_uuid} [delete]
func (h *AgentHandler) DeleteSession(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	sessionUUID := ctx.Param("session_uuid")
	if sessionUUID == "" {
		httpbase.BadRequest(ctx, "Invalid session UUID")
		return
	}

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	if err := h.agent.DeleteSessionByUUID(ctx.Request.Context(), currentUserUUID, sessionUUID, instanceID); err != nil {
		//check for permission error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to delete session by session uuid", "session_uuid", sessionUUID, "user_uuid", currentUserUUID, "error", err)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to delete session by session uuid", "session_uuid", sessionUUID, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// update session by session uuid
// @Security     ApiKey
// @Summary      Update session by session UUID
// @Description  Update a session by session UUID
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        session_uuid path string true "Session UUID"
// @Param        body body types.UpdateAgentInstanceSessionRequest true "Session data"
// @Success      200  {object}  types.Response{data=nil} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions/{session_uuid} [put]
func (h *AgentHandler) UpdateSession(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	sessionUUID := ctx.Param("session_uuid")
	if sessionUUID == "" {
		httpbase.BadRequest(ctx, "Invalid session UUID")
		return
	}

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	var req types.UpdateAgentInstanceSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if err := h.agent.UpdateSessionByUUID(ctx.Request.Context(), currentUserUUID, sessionUUID, instanceID, &req); err != nil {
		//check for permission error
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to update session by session uuid", "session_uuid", sessionUUID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to update session by session uuid", "session_uuid", sessionUUID, "user_uuid", currentUserUUID, "request", req, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// ListSessionHistories godoc
// @Security     ApiKey
// @Summary      List session histories by session UUID
// @Description  List all session histories for a specific session
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        session_uuid path string true "Session UUID"
// @Success      200  {object}  types.Response{data=[]types.AgentInstanceSessionHistory} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions/{session_uuid}/histories [get]
func (h *AgentHandler) ListSessionHistories(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	sessionUUID := ctx.Param("session_uuid")
	if sessionUUID == "" {
		httpbase.BadRequest(ctx, "Invalid session UUID")
		return
	}

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	histories, err := h.agent.ListSessionHistories(ctx.Request.Context(), currentUserUUID, sessionUUID, instanceID)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to list session histories by session uuid", "session_uuid", sessionUUID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to list session histories by session uuid", "session_uuid", sessionUUID, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, histories)
}

// CreateSessionHistory creates session histories for an agent instance session
// @Security     ApiKey
// @Summary      Create session histories for an agent instance session
// @Description  Create session histories for an agent instance session
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        session_uuid path string true "Session UUID"
// @Param        body body types.CreateSessionHistoryRequest true "Session history"
// @Success      200  {object}  types.Response{data=types.CreateSessionHistoryResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions/{session_uuid}/histories [post]
func (h *AgentHandler) CreateSessionHistory(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	sessionUUID := ctx.Param("session_uuid")
	if sessionUUID == "" {
		httpbase.BadRequest(ctx, "session_uuid is required")
		return
	}

	var req types.CreateSessionHistoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.SessionUUID = sessionUUID

	response, err := h.agent.CreateSessionHistories(ctx.Request.Context(), currentUserUUID, instanceID, &req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to create session history", "session_uuid", sessionUUID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to create session history", "session_uuid", sessionUUID, "user_uuid", currentUserUUID, "message", req.Messages, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, *response)
}

// UpdateSessionHistoryFeedback updates the feedback of a session history message
// @Security     ApiKey
// @Summary      Update the feedback of a session history message
// @Description  Update the feedback of a session history message
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        session_uuid path string true "Session UUID"
// @Param        msg_uuid path string true "Message UUID"
// @Param        body body types.FeedbackSessionHistoryRequest true "feedback for session history message"
// @Success      200  {object}  types.Response{data=nil} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions/{session_uuid}/histories/{msg_uuid}/feedback [put]
func (h *AgentHandler) UpdateSessionHistoryFeedback(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	sessionUUID := ctx.Param("session_uuid")
	if sessionUUID == "" {
		httpbase.BadRequest(ctx, "session_uuid is required")
		return
	}

	msgUUID := ctx.Param("msg_uuid")
	if msgUUID == "" {
		httpbase.BadRequest(ctx, "msg_uuid is required")
		return
	}

	var req types.FeedbackSessionHistoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.MsgUUID = msgUUID

	if err := h.agent.UpdateSessionHistoryFeedback(ctx.Request.Context(), currentUserUUID, instanceID, sessionUUID, &req); err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to update session history feedback", "session_uuid", sessionUUID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to update session history feedback", "session_uuid", sessionUUID, "user_uuid", currentUserUUID, "msg_uuid", msgUUID, "request", req, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// RewriteMessage rewrites an output message
// @Security     ApiKey
// @Summary      Rewrite an output message
// @Description  Rewrite an output message
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Instance ID"
// @Param        session_uuid path string true "Session UUID"
// @Param        msg_uuid path string true "Message UUID"
// @Param        body body types.RewriteSessionHistoryRequest true "Rewrite session history request"
// @Success      200  {object}  types.Response{data=types.RewriteSessionHistoryResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/instances/{id}/sessions/{session_uuid}/histories/{msg_uuid}/rewrite [put]
func (h *AgentHandler) RewriteMessage(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)

	// Extract instance_id from URL path parameter
	instanceIDStr := ctx.Param("id")
	if instanceIDStr == "" {
		httpbase.BadRequest(ctx, "Instance ID is required")
		return
	}

	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid instance ID: %s", instanceIDStr))
		return
	}

	sessionUUID := ctx.Param("session_uuid")
	if sessionUUID == "" {
		httpbase.BadRequest(ctx, "session_uuid is required")
		return
	}

	originalMsgUUID := ctx.Param("msg_uuid")
	if originalMsgUUID == "" {
		httpbase.BadRequest(ctx, "msg_uuid is required")
		return
	}

	var req types.RewriteSessionHistoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.OriginalMsgUUID = originalMsgUUID

	response, err := h.agent.RewriteSessionHistory(ctx.Request.Context(), currentUserUUID, instanceID, sessionUUID, &req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to rewrite message", "session_uuid", sessionUUID, "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to rewrite message", "session_uuid", sessionUUID, "user_uuid", currentUserUUID, "original_msg_uuid", originalMsgUUID, "request", req, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, *response)
}

// ListTasks godoc
// @Security     ApiKey
// @Summary      List agent tasks
// @Description  List all agent tasks for the current user with filtering and pagination
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        search query string false "search by task name"
// @Param        task_type query string false "filter by task type" Enums(finetuneJob, inference)
// @Param        status query string false "filter by status" Enums(in_progress, completed, failed)
// @Param        instance_id query int false "filter by instance ID"
// @Param        session_uuid query string false "filter by session UUID"
// @Param        per query int false "page size" default(50)
// @Param        page query int false "current page number" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.AgentTaskListItem,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/tasks [get]
func (h *AgentHandler) ListTasks(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var filter types.AgentTaskFilter
	filter.Search = ctx.Query("search")
	taskTypeStr := ctx.Query("task_type")
	if taskTypeStr != "" {
		filter.TaskType = types.AgentTaskType(taskTypeStr)
	}
	statusStr := ctx.Query("status")
	if statusStr != "" {
		filter.Status = types.AgentTaskStatus(statusStr)
	}
	instanceIDStr := ctx.Query("instance_id")
	if instanceIDStr != "" {
		var instanceID int64
		if _, err := fmt.Sscanf(instanceIDStr, "%d", &instanceID); err == nil {
			filter.InstanceID = &instanceID
		}
	}
	filter.SessionUUID = ctx.Query("session_uuid")

	tasks, total, err := h.agent.ListTasks(ctx.Request.Context(), currentUserUUID, filter, per, page)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("Forbidden to list agent tasks", "user_uuid", currentUserUUID)
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to list agent tasks", "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, tasks, total)
}

// GetTaskDetail godoc
// @Security     ApiKey
// @Summary      Get agent task detail
// @Description  Get detailed information about a specific agent task
// @Tags         Agent
// @Accept       json
// @Produce      json
// @Param        id path int true "Task ID (primary key)"
// @Success      200  {object}  types.Response{data=types.AgentTaskDetail} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      404  {object}  error "Task not found"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /agent/tasks/{id} [get]
func (h *AgentHandler) GetTaskDetail(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	idStr := ctx.Param("id")
	if idStr == "" {
		httpbase.BadRequest(ctx, "id is required")
		return
	}

	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		httpbase.BadRequest(ctx, "invalid id format")
		return
	}

	detail, err := h.agent.GetTaskDetail(ctx.Request.Context(), currentUserUUID, id)
	if err != nil {
		if errors.Is(err, errorx.ErrDatabaseNoRows) {
			slog.Info("Task not found", "id", id, "user_uuid", currentUserUUID)
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.Error("Failed to get task detail", "id", id, "user_uuid", currentUserUUID, "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}
