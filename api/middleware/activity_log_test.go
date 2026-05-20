package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
)

func TestMatchRule(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		wantAction  string
		wantResType string
		wantNil     bool
	}{
		// model upload/download
		{name: "model_upload_file", method: "POST", path: "/api/v1/models/ns/name/upload_file", wantAction: "upload_model_file", wantResType: "models"},
		{name: "model_upload_raw", method: "POST", path: "/api/v1/models/ns/name/raw/main/file.py", wantNil: true},
		{name: "model_lfs_upload", method: "PUT", path: "/hf/models/ns/name/info/lfs/objects/abc/123", wantAction: "upload_model_file", wantResType: "models"},
		{name: "model_lfs_batch", method: "POST", path: "/hf/models/ns/name/info/lfs/objects/batch", wantAction: "upload_model", wantResType: "models"},
		{name: "model_download", method: "GET", path: "/api/v1/models/ns/name/download/main/model.bin", wantAction: "download_model_file", wantResType: "models"},
		{name: "model_resolve", method: "GET", path: "/api/v1/models/ns/name/resolve/main/config.json", wantAction: "download_model_file", wantResType: "models"},
		{name: "model_blob", method: "GET", path: "/api/v1/models/ns/name/blob/main/readme.md", wantNil: true},
		{name: "model_lfs_download", method: "GET", path: "/hf/models/ns/name/info/lfs/abc", wantNil: true},
		// model inference
		{name: "deploy_inference", method: "POST", path: "/api/v1/models/ns/name/run", wantAction: "deploy_inference", wantResType: "models"},
		{name: "delete_inference", method: "DELETE", path: "/api/v1/models/ns/name/run/123", wantAction: "delete_inference", wantResType: "models"},
		{name: "stop_inference", method: "PUT", path: "/api/v1/models/ns/name/run/123/stop", wantAction: "stop_inference", wantResType: "models"},
		{name: "start_inference", method: "PUT", path: "/api/v1/models/ns/name/run/123/start", wantAction: "start_inference", wantResType: "models"},
		{name: "wakeup_inference", method: "PUT", path: "/api/v1/models/ns/name/run/123/wakeup", wantAction: "wakeup_inference", wantResType: "models"},
		// model fine-tuning
		{name: "create_finetune", method: "POST", path: "/api/v1/models/ns/name/finetune", wantAction: "create_finetune", wantResType: "models"},
		{name: "stop_finetune", method: "PUT", path: "/api/v1/models/ns/name/finetune/123/stop", wantAction: "stop_finetune", wantResType: "models"},
		{name: "start_finetune", method: "PUT", path: "/api/v1/models/ns/name/finetune/123/start", wantAction: "start_finetune", wantResType: "models"},
		{name: "delete_finetune", method: "DELETE", path: "/api/v1/models/ns/name/finetune/123", wantAction: "delete_finetune", wantResType: "models"},
		// model serverless
		{name: "deploy_serverless", method: "POST", path: "/api/v1/models/ns/name/serverless", wantAction: "deploy_serverless", wantResType: "models"},
		// dataset upload/download
		{name: "dataset_upload", method: "POST", path: "/api/v1/datasets/ns/name/upload_file", wantAction: "upload_dataset_file", wantResType: "datasets"},
		{name: "dataset_download", method: "GET", path: "/api/v1/datasets/ns/name/download/main/data.csv", wantAction: "download_dataset_file", wantResType: "datasets"},
		// dataset purchase
		{name: "buy_dataset", method: "POST", path: "/api/v1/datasets/ns/name/buy", wantAction: "buy_dataset", wantResType: "datasets"},
		{name: "refork_dataset", method: "POST", path: "/api/v1/datasets/ns/name/refork", wantNil: true},
		// agent
		{name: "create_agent_instance", method: "POST", path: "/api/v1/agent/instances", wantAction: "create_agent_instance", wantResType: "agent"},
		{name: "create_agent_template", method: "POST", path: "/api/v1/agent/templates", wantAction: "create_agent_template", wantResType: "agent"},
		{name: "create_agent_session", method: "POST", path: "/api/v1/agent/instances/123/sessions", wantAction: "create_agent_session", wantResType: "agent"},
		// should NOT match
		{name: "model_create", method: "POST", path: "/api/v1/models", wantNil: true},
		{name: "model_update", method: "PUT", path: "/api/v1/models/ns/name", wantNil: true},
		{name: "model_list", method: "GET", path: "/api/v1/models", wantNil: true},
		{name: "model_view", method: "GET", path: "/api/v1/models/ns/name", wantNil: true},
		{name: "code_upload", method: "POST", path: "/api/v1/codes/ns/name/upload_file", wantNil: true},
		{name: "user_get", method: "GET", path: "/api/v1/user/test", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, resType := matchRule(tt.path, tt.method)
			if tt.wantNil {
				assert.Nil(t, rule, "expected nil rule")
			} else {
				assert.NotNil(t, rule, "expected non-nil rule")
				if rule != nil {
					assert.Equal(t, tt.wantAction, rule.action)
					assert.Equal(t, tt.wantResType, resType)
				}
			}
		})
	}
}

func TestBuildActivityLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/models/ns/mymodel/run", nil)
	c.Params = []gin.Param{
		{Key: "namespace", Value: "ns"},
		{Key: "name", Value: "mymodel"},
	}

	result := buildActivityLog(c, "testuser", "uuid", httpbase.AuthTypeAccessToken)
	assert.NotNil(t, result)
	assert.Equal(t, "deploy_inference", result.Action)
	assert.Equal(t, "models", result.ResourceType)
	assert.Equal(t, "ns/mymodel", result.ResourceName)
	assert.Equal(t, "testuser", result.Username)
	assert.Equal(t, "uuid", result.UserID)
	assert.Equal(t, string(httpbase.AuthTypeAccessToken), result.AuthType)
}

func TestActivityLogMiddleware_NoUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ActivityLog(&config.Config{}, nil))
	r.POST("/api/v1/models/ns/name/run", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("POST", "/api/v1/models/ns/name/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
