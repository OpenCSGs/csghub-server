package middleware

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type actionRule struct {
	method       string
	pathContains []string // all must be contained in the URL path
	action       string
}

var modelActions = []actionRule{
	// upload / download
	{method: "POST", pathContains: []string{"/models/", "/upload_file"}, action: "upload_model_file"},
	{method: "POST", pathContains: []string{"/models/", "/info/lfs/objects/batch"}, action: "upload_model"},
	{method: "PUT", pathContains: []string{"/models/", "/info/lfs/objects/"}, action: "upload_model_file"},
	{method: "GET", pathContains: []string{"/models/", "/resolve/"}, action: "download_model_file"},
	{method: "GET", pathContains: []string{"/models/", "/download/"}, action: "download_model_file"},
	{method: "POST", pathContains: []string{"/models/", "/git-receive-pack"}, action: "push_model"},
	{method: "POST", pathContains: []string{"/models/", "/git-upload-pack"}, action: "clone_model"},
	// deploy / inference
	{method: "POST", pathContains: []string{"/models/", "/run"}, action: "deploy_inference"},
	{method: "DELETE", pathContains: []string{"/models/", "/run/"}, action: "delete_inference"},
	{method: "PUT", pathContains: []string{"/models/", "/run/", "/stop"}, action: "stop_inference"},
	{method: "PUT", pathContains: []string{"/models/", "/run/", "/start"}, action: "start_inference"},
	{method: "PUT", pathContains: []string{"/models/", "/run/", "/wakeup"}, action: "wakeup_inference"},
	// fine-tuning
	{method: "POST", pathContains: []string{"/models/", "/finetune"}, action: "create_finetune"},
	{method: "PUT", pathContains: []string{"/models/", "/finetune/", "/stop"}, action: "stop_finetune"},
	{method: "PUT", pathContains: []string{"/models/", "/finetune/", "/start"}, action: "start_finetune"},
	{method: "DELETE", pathContains: []string{"/models/", "/finetune/"}, action: "delete_finetune"},
	{method: "POST", pathContains: []string{"/models/", "/finetunes"}, action: "run_finetune_job"},
	{method: "POST", pathContains: []string{"/models/", "/evaluations"}, action: "run_evaluation_job"},
	// serverless
	{method: "POST", pathContains: []string{"/models/", "/serverless"}, action: "deploy_serverless"},
}

var datasetActions = []actionRule{
	// upload / download
	{method: "POST", pathContains: []string{"/datasets/", "/upload_file"}, action: "upload_dataset_file"},
	{method: "GET", pathContains: []string{"/datasets/", "/resolve/"}, action: "download_dataset_file"},
	{method: "GET", pathContains: []string{"/datasets/", "/download/"}, action: "download_dataset_file"},
	{method: "POST", pathContains: []string{"/datasets/", "/git-receive-pack"}, action: "push_dataset"},
	{method: "POST", pathContains: []string{"/datasets/", "/git-upload-pack"}, action: "clone_dataset"},
	// purchase
	{method: "POST", pathContains: []string{"/datasets/", "/buy"}, action: "buy_dataset"},
}

var agentActions = []actionRule{
	{method: "POST", pathContains: []string{"/agent/instances/", "/sessions"}, action: "create_agent_session"},
	{method: "POST", pathContains: []string{"/agent/instances"}, action: "create_agent_instance"},
	{method: "POST", pathContains: []string{"/agent/templates"}, action: "create_agent_template"},
}

func ActivityLog(config *config.Config, comp component.ActivityLogComponent) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !config.ActivityLog.Enabled {
			return
		}

		username := httpbase.GetCurrentUser(c)
		if username == "" {
			return
		}

		userUUID := httpbase.GetCurrentUserUUID(c)
		if userUUID == "" {
			return
		}

		authType := httpbase.GetAuthType(c)

		logEntry := buildActivityLog(c, username, userUUID, authType)
		if logEntry == nil {
			return
		}

		if err := comp.PublishActivityLog(c.Request.Context(), logEntry); err != nil {
			slog.Debug("failed to publish activity log", slog.Any("error", err))
		}
	}
}

func buildActivityLog(c *gin.Context, username, userUUID string, authType httpbase.AuthType) *types.ActivityLog {
	path := c.Request.URL.Path
	method := c.Request.Method

	rule, resourceType := matchRule(path, method)
	if rule == nil {
		return nil
	}

	resourceName := c.Param("namespace")
	if name := c.Param("name"); name != "" {
		resourceName = resourceName + "/" + name
	}

	var resourceID int64
	if idStr := c.Param("id"); idStr != "" {
		resourceID, _ = strconv.ParseInt(idStr, 10, 64)
	}

	return &types.ActivityLog{
		Username:      username,
		UserID:        userUUID,
		AuthType:      string(authType),
		Action:        rule.action,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		ResourceName:  resourceName,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
		OperationTime: time.Now(),
	}
}

func matchRule(path, method string) (*actionRule, string) {
	allRules := []struct {
		rules        []actionRule
		resourceType string
	}{
		{modelActions, "models"},
		{datasetActions, "datasets"},
		{agentActions, "agent"},
	}

	for _, group := range allRules {
		for i := range group.rules {
			rule := &group.rules[i]
			if rule.method != method {
				continue
			}
			allMatch := true
			for _, sub := range rule.pathContains {
				if !strings.Contains(path, sub) {
					allMatch = false
					break
				}
			}
			if allMatch {
				return rule, group.resourceType
			}
		}
	}

	return nil, ""
}
