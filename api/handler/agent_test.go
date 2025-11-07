package handler

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AgentServerHandlerTester struct {
	*testutil.GinTester
	handler *AgentHandler
	mocks   struct {
		agent *mockcomponent.MockAgentComponent
	}
}

func NewAgentServerHandlerTester(t *testing.T) *AgentServerHandlerTester {
	tester := &AgentServerHandlerTester{
		GinTester: testutil.NewGinTester(),
	}
	tester.mocks.agent = mockcomponent.NewMockAgentComponent(t)
	tester.handler = &AgentHandler{
		agent: tester.mocks.agent,
	}
	return tester
}

func (t *AgentServerHandlerTester) WithHandleFunc(fn func(h *AgentHandler) gin.HandlerFunc) *AgentServerHandlerTester {
	t.Handler(fn(t.handler))
	return t
}

// Template Tests

func TestAgentHandler_CreateTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateTemplate
		})
		tester.WithKV("currentUserUUID", "u")

		templateType := "langflow"
		content := "test template content"
		name := "Test Template"
		description := "Test template description"
		metadata := map[string]any{"tags": []any{"CSGHub", "openai", "q-a"}}
		req := &types.AgentTemplate{
			Type:        &templateType,
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(false),
			Metadata:    &metadata,
		}

		// Mock expectation - the component should receive the template with UserUUID set
		expectedTemplate := &types.AgentTemplate{
			Type:        &templateType,
			UserUUID:    &[]string{"u"}[0], // Current user from WithUser()
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(false),
			Metadata:    &metadata,
		}

		tester.mocks.agent.EXPECT().CreateTemplate(tester.Ctx(), expectedTemplate).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedTemplate)
	})

	t.Run("validation error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateTemplate
		})
		tester.WithKV("currentUserUUID", "u")

		// Invalid request - missing required fields
		req := &types.AgentTemplate{}

		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateTemplate
		})
		tester.WithKV("currentUserUUID", "u")

		templateType := "langflow"
		content := "test template content"
		name := "Test Template"
		description := "Test template description"
		metadata := map[string]any{"tags": []any{"CSGHub", "openai", "q-a"}}
		req := &types.AgentTemplate{
			Type:        &templateType,
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(false),
			Metadata:    &metadata,
		}

		expectedTemplate := &types.AgentTemplate{
			Type:        &templateType,
			UserUUID:    &[]string{"u"}[0],
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(false),
			Metadata:    &metadata,
		}

		tester.mocks.agent.EXPECT().CreateTemplate(tester.Ctx(), expectedTemplate).Return(errors.New("database error"))
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}

func TestAgentHandler_GetTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		templateType := "langflow"
		content := "test template content"
		userUUID := "u"
		name := "Test Template"
		description := "Test template description"
		metadata := map[string]any{"tags": []any{"CSGHub", "openai", "q-a"}}
		expectedTemplate := &types.AgentTemplate{
			ID:          templateID,
			Type:        &templateType,
			UserUUID:    &userUUID,
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(false),
			Metadata:    &metadata,
		}

		tester.mocks.agent.EXPECT().GetTemplateByID(tester.Ctx(), templateID, "u").Return(expectedTemplate, nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedTemplate)
	})

	t.Run("invalid template ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid")

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		tester.mocks.agent.EXPECT().GetTemplateByID(tester.Ctx(), templateID, "u").Return(nil, errorx.ErrForbidden)
		tester.Execute()

		tester.ResponseEqSimple(t, 403, map[string]interface{}{"code": "AUTH-ERR-2", "msg": "AUTH-ERR-2"})
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		tester.mocks.agent.EXPECT().GetTemplateByID(tester.Ctx(), templateID, "u").Return(nil, errors.New("database error"))
		tester.Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}

func TestAgentHandler_ListTemplates(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListTemplates
		})
		tester.WithKV("currentUserUUID", "u").WithQuery("per", "10").WithQuery("page", "1")

		templateType := "langflow"
		content := "test template content"
		userUUID := "u"
		name := "Test Template"
		description := "Test template description"
		expectedTemplates := []types.AgentTemplate{
			{
				ID:          1,
				Type:        &templateType,
				UserUUID:    &userUUID,
				Name:        &name,
				Description: &description,
				Content:     &content,
				Public:      boolPtr(false),
				Metadata:    &map[string]any{"tags": []any{"CSGHub"}},
			},
		}

		tester.mocks.agent.EXPECT().ListTemplatesByUserUUID(tester.Ctx(), "u", mock.MatchedBy(func(filter types.AgentTemplateFilter) bool {
			return filter.Search == "" && filter.Type == ""
		}), 10, 1).Return(expectedTemplates, 1, nil)
		tester.Execute()

		// Create expected response structure that matches OKWithTotal
		expectedResponse := gin.H{
			"msg":   "OK",
			"data":  expectedTemplates,
			"total": 1,
		}
		tester.ResponseEqSimple(t, 200, expectedResponse)
	})

	t.Run("success with query parameters", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListTemplates
		})
		tester.WithKV("currentUserUUID", "u").WithQuery("search", "langflow").WithQuery("type", "langflow").WithQuery("per", "10").WithQuery("page", "1")

		templateType := "langflow"
		content := "test template content"
		userUUID := "u"
		name := "Langflow Template"
		description := "A langflow template"
		expectedTemplates := []types.AgentTemplate{
			{
				ID:          1,
				Type:        &templateType,
				UserUUID:    &userUUID,
				Name:        &name,
				Description: &description,
				Content:     &content,
				Public:      boolPtr(false),
				Metadata:    &map[string]any{"tags": []any{"langflow"}},
			},
		}

		tester.mocks.agent.EXPECT().ListTemplatesByUserUUID(tester.Ctx(), "u", mock.MatchedBy(func(filter types.AgentTemplateFilter) bool {
			return filter.Search == "langflow" && filter.Type == "langflow"
		}), 10, 1).Return(expectedTemplates, 1, nil)
		tester.Execute()

		// Create expected response structure that matches OKWithTotal
		expectedResponse := gin.H{
			"msg":   "OK",
			"data":  expectedTemplates,
			"total": 1,
		}
		tester.ResponseEqSimple(t, 200, expectedResponse)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListTemplates
		})
		tester.WithKV("currentUserUUID", "u").WithQuery("per", "10").WithQuery("page", "1")

		tester.mocks.agent.EXPECT().ListTemplatesByUserUUID(tester.Ctx(), "u", mock.Anything, 10, 1).Return(nil, 0, errors.New("database error"))
		tester.Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})

	t.Run("bad request format", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListTemplates
		})
		tester.WithKV("currentUserUUID", "u").WithQuery("per", "invalid")

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})
}

func TestAgentHandler_UpdateTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		templateType := "langflow"
		content := "updated content"
		userUUID := "u"
		name := "Updated Template"
		description := "Updated template description"
		metadata := map[string]any{"tags": []any{"CSGHub", "openai", "q-a"}}
		req := &types.AgentTemplate{
			Type:        &templateType,
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(true),
			Metadata:    &metadata,
		}

		expectedTemplate := &types.AgentTemplate{
			ID:          templateID,
			Type:        &templateType,
			UserUUID:    &userUUID,
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(true),
			Metadata:    &metadata,
		}

		tester.mocks.agent.EXPECT().UpdateTemplate(tester.Ctx(), expectedTemplate).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedTemplate)
	})

	t.Run("invalid template ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid")

		req := &types.AgentTemplate{}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		templateType := "langflow"
		content := "updated content"
		userUUID := "u"
		name := "Updated Template"
		description := "Updated template description"
		req := &types.AgentTemplate{
			Type:        &templateType,
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(true),
		}

		expectedTemplate := &types.AgentTemplate{
			ID:          templateID,
			Type:        &templateType,
			UserUUID:    &userUUID,
			Name:        &name,
			Description: &description,
			Content:     &content,
			Public:      boolPtr(true),
		}

		tester.mocks.agent.EXPECT().UpdateTemplate(tester.Ctx(), expectedTemplate).Return(errorx.ErrForbidden)
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 403)
	})

	t.Run("success - update metadata", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		templateType := "langflow"
		userUUID := "u"
		newMetadata := map[string]any{
			"key1": "new-value1",
			"key2": 789,
			"key4": "new-key",
		}
		// Name is required by validation, but we can use a placeholder for metadata-only updates
		name := "Template"
		req := &types.AgentTemplate{
			Type:     &templateType,
			Name:     &name,
			Metadata: &newMetadata,
		}

		// JSON unmarshaling converts numbers to float64, so we need to match that
		expectedMetadata := map[string]any{
			"key1": "new-value1",
			"key2": float64(789),
			"key4": "new-key",
		}
		expectedTemplate := &types.AgentTemplate{
			ID:       templateID,
			Type:     &templateType,
			UserUUID: &userUUID,
			Name:     &name,
			Metadata: &expectedMetadata,
		}

		tester.mocks.agent.EXPECT().UpdateTemplate(tester.Ctx(), expectedTemplate).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedTemplate)
	})

	t.Run("success - delete metadata keys", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		templateType := "langflow"
		userUUID := "u"
		// Set value to nil to delete the key
		newMetadata := map[string]any{
			"key1": nil,
			"key2": nil,
		}
		// Name is required by validation, but we can use a placeholder for metadata-only updates
		name := "Template"
		req := &types.AgentTemplate{
			Type:     &templateType,
			Name:     &name,
			Metadata: &newMetadata,
		}

		expectedTemplate := &types.AgentTemplate{
			ID:       templateID,
			Type:     &templateType,
			UserUUID: &userUUID,
			Name:     &name,
			Metadata: &newMetadata,
		}

		tester.mocks.agent.EXPECT().UpdateTemplate(tester.Ctx(), expectedTemplate).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedTemplate)
	})

	t.Run("success - mixed metadata operations", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		templateType := "langflow"
		userUUID := "u"
		name := "Updated Template"
		// Mixed operations: update key1, delete key2, add key3
		newMetadata := map[string]any{
			"key1": "new-value1",
			"key2": nil,
			"key3": "new-key3",
		}
		req := &types.AgentTemplate{
			Type:     &templateType,
			Name:     &name,
			Metadata: &newMetadata,
		}

		expectedTemplate := &types.AgentTemplate{
			ID:       templateID,
			Type:     &templateType,
			UserUUID: &userUUID,
			Name:     &name,
			Metadata: &newMetadata,
		}

		tester.mocks.agent.EXPECT().UpdateTemplate(tester.Ctx(), expectedTemplate).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedTemplate)
	})
}

func TestAgentHandler_DeleteTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		tester.mocks.agent.EXPECT().DeleteTemplate(tester.Ctx(), templateID, "u").Return(nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("invalid template ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid")

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		templateID := int64(1)
		tester.mocks.agent.EXPECT().DeleteTemplate(tester.Ctx(), templateID, "u").Return(errorx.ErrForbidden)
		tester.Execute()

		tester.ResponseEqCode(t, 403)
	})
}

// Instance Tests

func TestAgentHandler_CreateInstance(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateInstance
		})
		tester.WithKV("currentUserUUID", "u")

		templateID := int64(1)
		instanceType := "langflow"
		instanceName := "test instance"
		description := "test description"
		req := &types.AgentInstance{
			TemplateID:  &templateID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
		}

		expectedInstance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &[]string{"u"}[0],
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
		}

		expectedResult := &types.AgentInstance{
			ID:          0,
			TemplateID:  &templateID,
			UserUUID:    &[]string{"u"}[0],
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			ContentID:   nil,
			Public:      boolPtr(false),
			Editable:    false,
		}

		tester.mocks.agent.EXPECT().CreateInstance(tester.Ctx(), expectedInstance).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedResult)
	})

	t.Run("validation error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateInstance
		})
		tester.WithKV("currentUserUUID", "u")

		// Invalid request - missing required fields
		req := &types.AgentInstance{}

		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateInstance
		})
		tester.WithKV("currentUserUUID", "u")

		templateID := int64(1)
		instanceType := "langflow"
		instanceName := "test instance"
		description := "test description"
		req := &types.AgentInstance{
			TemplateID:  &templateID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
		}

		expectedInstance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &[]string{"u"}[0],
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
		}

		tester.mocks.agent.EXPECT().CreateInstance(tester.Ctx(), expectedInstance).Return(errorx.ErrForbidden)
		tester.WithBody(t, req).Execute()

		tester.ResponseEqSimple(t, 403, map[string]interface{}{"code": "AUTH-ERR-2", "msg": "AUTH-ERR-2"})
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateInstance
		})
		tester.WithKV("currentUserUUID", "u")

		templateID := int64(1)
		instanceType := "langflow"
		instanceName := "test instance"
		description := "test description"
		req := &types.AgentInstance{
			TemplateID:  &templateID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
		}

		expectedInstance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &[]string{"u"}[0],
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
		}

		tester.mocks.agent.EXPECT().CreateInstance(tester.Ctx(), expectedInstance).Return(errors.New("database error"))
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})

	t.Run("success - no template", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateInstance
		})
		tester.WithKV("currentUserUUID", "u")

		instanceType := "langflow"
		instanceName := "test instance"
		description := "test description"
		req := &types.AgentInstance{
			TemplateID:  nil, // No template
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
		}

		expectedInstance := &types.AgentInstance{
			TemplateID:  nil, // No template
			UserUUID:    &[]string{"u"}[0],
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
		}

		expectedResult := &types.AgentInstance{
			ID:          0,
			TemplateID:  nil, // No template
			UserUUID:    &[]string{"u"}[0],
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			ContentID:   nil,
			Public:      boolPtr(false),
			Editable:    false,
		}

		tester.mocks.agent.EXPECT().CreateInstance(tester.Ctx(), expectedInstance).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedResult)
	})
}

func TestAgentHandler_GetInstance(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		instanceType := "langflow"
		instanceName := "test instance"
		description := "test description"
		userUUID := "u"
		expectedInstance := &types.AgentInstance{
			ID:          instanceID,
			Type:        &instanceType,
			UserUUID:    &userUUID,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(false),
			Editable:    true, // Owner should be able to edit
		}

		tester.mocks.agent.EXPECT().GetInstanceByID(tester.Ctx(), instanceID, "u").Return(expectedInstance, nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedInstance)
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid")

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		tester.mocks.agent.EXPECT().GetInstanceByID(tester.Ctx(), instanceID, "u").Return(nil, errorx.ErrForbidden)
		tester.Execute()

		tester.ResponseEqSimple(t, 403, map[string]interface{}{"code": "AUTH-ERR-2", "msg": "AUTH-ERR-2"})
	})
}

func TestAgentHandler_ListInstances(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListInstances
		})
		tester.WithKV("currentUserUUID", "u").WithQuery("per", "10").WithQuery("page", "1")

		instanceType := "langflow"
		instanceName := "test instance"
		description := "test description"
		userUUID := "u"
		expectedInstances := []*types.AgentInstance{
			{
				ID:          1,
				Type:        &instanceType,
				UserUUID:    &userUUID,
				Name:        &instanceName,
				Description: &description,
				Public:      boolPtr(false),
				Editable:    true, // Owner should be able to edit
			},
		}

		tester.mocks.agent.EXPECT().ListInstancesByUserUUID(tester.Ctx(), "u", mock.MatchedBy(func(filter types.AgentInstanceFilter) bool {
			return filter.Search == "" && filter.Type == "" && filter.TemplateID == nil
		}), 10, 1).Return(expectedInstances, 1, nil)
		tester.Execute()

		// Create expected response structure that matches OKWithTotal
		expectedResponse := gin.H{
			"msg":   "OK",
			"data":  expectedInstances,
			"total": 1,
		}
		tester.ResponseEqSimple(t, 200, expectedResponse)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListInstances
		})
		tester.WithKV("currentUserUUID", "u").WithQuery("per", "10").WithQuery("page", "1")

		tester.mocks.agent.EXPECT().ListInstancesByUserUUID(tester.Ctx(), "u", mock.MatchedBy(func(filter types.AgentInstanceFilter) bool {
			return filter.Search == "" && filter.Type == "" && filter.TemplateID == nil
		}), 10, 1).Return(nil, 0, errors.New("database error"))
		tester.Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}

func TestAgentHandler_ListInstancesByTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListInstancesByTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1").WithQuery("per", "10").WithQuery("page", "1")

		instanceType := "langflow"
		instanceName := "test instance"
		description := "test description"
		userUUID := "u"
		expectedInstances := []*types.AgentInstance{
			{
				ID:          1,
				Type:        &instanceType,
				UserUUID:    &userUUID,
				Name:        &instanceName,
				Description: &description,
				Public:      boolPtr(false),
			},
		}

		tester.mocks.agent.EXPECT().ListInstancesByUserUUID(tester.Ctx(), "u", mock.MatchedBy(func(filter types.AgentInstanceFilter) bool {
			return filter.TemplateID != nil && *filter.TemplateID == 1
		}), 10, 1).Return(expectedInstances, 1, nil)
		tester.Execute()

		// Create expected response structure that matches OKWithTotal
		expectedResponse := gin.H{
			"msg":   "OK",
			"data":  expectedInstances,
			"total": 1,
		}
		tester.ResponseEqSimple(t, 200, expectedResponse)
	})

	t.Run("invalid template ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListInstancesByTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid")

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("success with query parameters", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListInstancesByTemplate
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1").WithQuery("search", "langflow").WithQuery("type", "langflow").WithQuery("per", "10").WithQuery("page", "1")

		instanceType := "langflow"
		instanceName := "test instance"
		description := "test description"
		userUUID := "u"
		expectedInstances := []*types.AgentInstance{
			{
				ID:          1,
				Type:        &instanceType,
				UserUUID:    &userUUID,
				Name:        &instanceName,
				Description: &description,
				Public:      boolPtr(false),
			},
		}

		tester.mocks.agent.EXPECT().ListInstancesByUserUUID(tester.Ctx(), "u", mock.MatchedBy(func(filter types.AgentInstanceFilter) bool {
			return filter.TemplateID != nil && *filter.TemplateID == 1 &&
				filter.Search == "langflow" && filter.Type == "langflow"
		}), 10, 1).Return(expectedInstances, 1, nil)
		tester.Execute()

		// Create expected response structure that matches OKWithTotal
		expectedResponse := gin.H{
			"msg":   "OK",
			"data":  expectedInstances,
			"total": 1,
		}
		tester.ResponseEqSimple(t, 200, expectedResponse)
	})
}

func TestAgentHandler_UpdateInstance(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		instanceType := "langflow"
		instanceName := "updated instance"
		description := "updated description"
		userUUID := "u"
		req := &types.AgentInstance{
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(true),
		}

		expectedInstance := &types.AgentInstance{
			ID:          instanceID,
			Type:        &instanceType,
			UserUUID:    &userUUID,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(true),
		}

		tester.mocks.agent.EXPECT().UpdateInstance(tester.Ctx(), expectedInstance).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid")

		req := &types.AgentInstance{}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		instanceType := "langflow"
		instanceName := "updated instance"
		description := "updated description"
		userUUID := "u"
		req := &types.AgentInstance{
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(true),
		}

		expectedInstance := &types.AgentInstance{
			ID:          instanceID,
			Type:        &instanceType,
			UserUUID:    &userUUID,
			Name:        &instanceName,
			Description: &description,
			Public:      boolPtr(true),
		}

		tester.mocks.agent.EXPECT().UpdateInstance(tester.Ctx(), expectedInstance).Return(errorx.ErrForbidden)
		tester.WithBody(t, req).Execute()

		tester.ResponseEqSimple(t, 403, map[string]interface{}{"code": "AUTH-ERR-2", "msg": "AUTH-ERR-2"})
	})

	t.Run("success - update metadata", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		instanceType := "langflow"
		userUUID := "u"
		newMetadata := map[string]any{
			"key1": "new-value1",
			"key2": 789,
			"key4": "new-key",
		}
		req := &types.AgentInstance{
			Type:     &instanceType,
			Metadata: &newMetadata,
		}

		// JSON unmarshaling converts numbers to float64, so we need to match that
		expectedMetadata := map[string]any{
			"key1": "new-value1",
			"key2": float64(789),
			"key4": "new-key",
		}
		expectedInstance := &types.AgentInstance{
			ID:       instanceID,
			Type:     &instanceType,
			UserUUID: &userUUID,
			Metadata: &expectedMetadata,
		}

		tester.mocks.agent.EXPECT().UpdateInstance(tester.Ctx(), expectedInstance).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("success - delete metadata keys", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		instanceType := "langflow"
		userUUID := "u"
		// Set value to nil to delete the key
		newMetadata := map[string]any{
			"key1": nil,
			"key2": nil,
		}
		req := &types.AgentInstance{
			Type:     &instanceType,
			Metadata: &newMetadata,
		}

		expectedInstance := &types.AgentInstance{
			ID:       instanceID,
			Type:     &instanceType,
			UserUUID: &userUUID,
			Metadata: &newMetadata,
		}

		tester.mocks.agent.EXPECT().UpdateInstance(tester.Ctx(), expectedInstance).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("success - mixed metadata operations", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		instanceType := "langflow"
		instanceName := "updated instance"
		userUUID := "u"
		// Mixed operations: update key1, delete key2, add key3
		newMetadata := map[string]any{
			"key1": "new-value1",
			"key2": nil,
			"key3": "new-key3",
		}
		req := &types.AgentInstance{
			Type:     &instanceType,
			Name:     &instanceName,
			Metadata: &newMetadata,
		}

		expectedInstance := &types.AgentInstance{
			ID:       instanceID,
			Type:     &instanceType,
			UserUUID: &userUUID,
			Name:     &instanceName,
			Metadata: &newMetadata,
		}

		tester.mocks.agent.EXPECT().UpdateInstance(tester.Ctx(), expectedInstance).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})
}

func TestAgentHandler_DeleteInstance(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		tester.mocks.agent.EXPECT().DeleteInstance(tester.Ctx(), instanceID, "u").Return(nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid")

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteInstance
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "1")

		instanceID := int64(1)
		tester.mocks.agent.EXPECT().DeleteInstance(tester.Ctx(), instanceID, "u").Return(errorx.ErrForbidden)
		tester.Execute()

		tester.ResponseEqCode(t, 403)
	})
}

func TestAgentHandler_UpdateInstanceByContentID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstanceByContentID
		})
		tester.WithKV("currentUserUUID", "u").WithParam("type", "langflow").WithParam("content_id", "content-123")

		updateRequest := types.UpdateAgentInstanceRequest{
			Name:        stringPtr("Updated Name"),
			Description: stringPtr("Updated Description"),
		}
		tester.WithBody(t, updateRequest)

		expectedInstance := &types.AgentInstance{
			ID:          1,
			TemplateID:  int64Ptr(1),
			UserUUID:    stringPtr("u"),
			Type:        stringPtr("langflow"),
			ContentID:   stringPtr("content-123"),
			Name:        stringPtr("Updated Name"),
			Description: stringPtr("Updated Description"),
			Public:      boolPtr(false),
		}

		tester.mocks.agent.EXPECT().UpdateInstanceByContentID(tester.Ctx(), "u", "langflow", "content-123", updateRequest).Return(expectedInstance, nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedInstance)
	})

	t.Run("validation error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstanceByContentID
		})
		tester.WithKV("currentUserUUID", "u").WithParam("type", "langflow").WithParam("content_id", "content-123")

		// Invalid JSON
		tester.WithBody(t, "invalid json")
		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstanceByContentID
		})
		tester.WithKV("currentUserUUID", "u").WithParam("type", "langflow").WithParam("content_id", "content-123")

		updateRequest := types.UpdateAgentInstanceRequest{
			Name: stringPtr("Updated Name"),
		}
		tester.WithBody(t, updateRequest)

		tester.mocks.agent.EXPECT().UpdateInstanceByContentID(tester.Ctx(), "u", "langflow", "content-123", updateRequest).Return(nil, errorx.ErrForbidden)
		tester.Execute()

		tester.ResponseEqCode(t, 403)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateInstanceByContentID
		})
		tester.WithKV("currentUserUUID", "u").WithParam("type", "langflow").WithParam("content_id", "content-123")

		updateRequest := types.UpdateAgentInstanceRequest{
			Name: stringPtr("Updated Name"),
		}
		tester.WithBody(t, updateRequest)

		tester.mocks.agent.EXPECT().UpdateInstanceByContentID(tester.Ctx(), "u", "langflow", "content-123", updateRequest).Return(nil, errors.New("database error"))
		tester.Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}

func TestAgentHandler_DeleteInstanceByContentID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteInstanceByContentID
		})
		tester.WithKV("currentUserUUID", "u").WithParam("type", "langflow").WithParam("content_id", "content-123")

		tester.mocks.agent.EXPECT().DeleteInstanceByContentID(tester.Ctx(), "u", "langflow", "content-123").Return(nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteInstanceByContentID
		})
		tester.WithKV("currentUserUUID", "u").WithParam("type", "langflow").WithParam("content_id", "content-123")

		tester.mocks.agent.EXPECT().DeleteInstanceByContentID(tester.Ctx(), "u", "langflow", "content-123").Return(errorx.ErrForbidden)
		tester.Execute()

		tester.ResponseEqCode(t, 403)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteInstanceByContentID
		})
		tester.WithKV("currentUserUUID", "u").WithParam("type", "langflow").WithParam("content_id", "content-123")

		tester.mocks.agent.EXPECT().DeleteInstanceByContentID(tester.Ctx(), "u", "langflow", "content-123").Return(errors.New("database error"))
		tester.Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}

// Helper functions for pointers
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

// Session Tests

func TestAgentHandler_CreateSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16")

		instanceID := int64(16)
		req := &types.CreateAgentInstanceSessionRequest{
			InstanceID: &instanceID,
		}

		expectedSessionUUID := "session-uuid-123"
		tester.mocks.agent.EXPECT().CreateSession(tester.Ctx(), "u", req).Return(expectedSessionUUID, nil)
		tester.WithBody(t, req).Execute()

		expectedResponse := types.CreateAgentInstanceSessionResponse{
			SessionUUID: expectedSessionUUID,
		}
		tester.ResponseEq(t, 200, tester.OKText, expectedResponse)
	})

	t.Run("validation error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16")

		// Invalid request - missing required fields
		req := &types.CreateAgentInstanceSessionRequest{}

		// The handler will set InstanceID from URL parameter
		expectedReq := &types.CreateAgentInstanceSessionRequest{
			InstanceID: int64Ptr(16),
		}

		// The handler will still call CreateSession even with invalid request
		tester.mocks.agent.EXPECT().CreateSession(tester.Ctx(), "u", expectedReq).Return("", errors.New("validation error"))
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 500, "validation error", nil)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16")

		instanceID := int64(16)
		req := &types.CreateAgentInstanceSessionRequest{
			InstanceID: &instanceID,
		}

		tester.mocks.agent.EXPECT().CreateSession(tester.Ctx(), "u", req).Return("", errors.New("database error"))
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}

func TestAgentHandler_ListSessions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListSessions
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithQuery("per", "10").WithQuery("page", "1")

		sessionUUID := "session-uuid-123"
		sessionName := "Test Session"
		userUUID := "u"
		instanceID := int64(16)
		expectedSessions := []*types.AgentInstanceSession{
			{
				ID:          1,
				SessionUUID: sessionUUID,
				Name:        sessionName,
				InstanceID:  instanceID,
				UserUUID:    userUUID,
				Type:        "langflow",
			},
		}
		total := 1

		tester.mocks.agent.EXPECT().ListSessions(tester.Ctx(), "u", mock.MatchedBy(func(filter types.AgentInstanceSessionFilter) bool {
			return filter.InstanceID != nil && *filter.InstanceID == 16
		}), 10, 1).Return(expectedSessions, total, nil)
		tester.Execute()

		expectedResponse := gin.H{
			"msg":   "OK",
			"data":  expectedSessions,
			"total": total,
		}
		tester.ResponseEqSimple(t, 200, expectedResponse)
	})

	t.Run("success with query parameters", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListSessions
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithQuery("type", "langflow").WithQuery("per", "10").WithQuery("page", "1")

		sessionUUID := "session-uuid-123"
		sessionName := "Test Session"
		userUUID := "u"
		instanceID := int64(16)
		expectedSessions := []*types.AgentInstanceSession{
			{
				ID:          1,
				SessionUUID: sessionUUID,
				Name:        sessionName,
				InstanceID:  instanceID,
				UserUUID:    userUUID,
				Type:        "langflow",
			},
		}
		total := 1

		tester.mocks.agent.EXPECT().ListSessions(tester.Ctx(), "u", mock.MatchedBy(func(filter types.AgentInstanceSessionFilter) bool {
			return filter.InstanceID != nil && *filter.InstanceID == 16
		}), 10, 1).Return(expectedSessions, total, nil)
		tester.Execute()

		expectedResponse := gin.H{
			"msg":   "OK",
			"data":  expectedSessions,
			"total": total,
		}
		tester.ResponseEqSimple(t, 200, expectedResponse)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListSessions
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithQuery("per", "10").WithQuery("page", "1")

		tester.mocks.agent.EXPECT().ListSessions(tester.Ctx(), "u", mock.Anything, 10, 1).Return(nil, 0, errors.New("database error"))
		tester.Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}

func TestAgentHandler_GetSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		sessionUUID := "session-uuid-123"
		sessionName := "Test Session"
		userUUID := "u"
		instanceID := int64(16)
		expectedSession := &types.AgentInstanceSession{
			ID:          1,
			SessionUUID: sessionUUID,
			Name:        sessionName,
			InstanceID:  instanceID,
			UserUUID:    userUUID,
			Type:        "langflow",
		}

		tester.mocks.agent.EXPECT().GetSessionByUUID(tester.Ctx(), "u", sessionUUID, instanceID).Return(expectedSession, nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedSession)
	})

	t.Run("session not found", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.GetSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "non-existent-session")

		tester.mocks.agent.EXPECT().GetSessionByUUID(tester.Ctx(), "u", "non-existent-session", int64(16)).Return(nil, errors.New("session not found"))
		tester.Execute()

		tester.ResponseEq(t, 500, "session not found", nil)
	})
}

func TestAgentHandler_UpdateSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		sessionUUID := "session-uuid-123"
		newName := "Updated Session Name"
		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		tester.mocks.agent.EXPECT().UpdateSessionByUUID(tester.Ctx(), "u", sessionUUID, int64(16), req).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("invalid session UUID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "")

		req := &types.UpdateAgentInstanceSessionRequest{
			Name: "Updated Session Name",
		}

		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("validation error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		// Invalid JSON
		tester.WithBody(t, "invalid json")
		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		sessionUUID := "session-uuid-123"
		newName := "Updated Session Name"
		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		tester.mocks.agent.EXPECT().UpdateSessionByUUID(tester.Ctx(), "u", sessionUUID, int64(16), req).Return(errorx.ErrForbidden)
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 403)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		sessionUUID := "session-uuid-123"
		newName := "Updated Session Name"
		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		tester.mocks.agent.EXPECT().UpdateSessionByUUID(tester.Ctx(), "u", sessionUUID, int64(16), req).Return(errors.New("database error"))
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}

func TestAgentHandler_DeleteSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		sessionUUID := "session-uuid-123"
		tester.mocks.agent.EXPECT().DeleteSessionByUUID(tester.Ctx(), "u", sessionUUID, int64(16)).Return(nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("session not found", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.DeleteSession
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "non-existent-session")

		tester.mocks.agent.EXPECT().DeleteSessionByUUID(tester.Ctx(), "u", "non-existent-session", int64(16)).Return(errors.New("session not found"))
		tester.Execute()

		tester.ResponseEq(t, 500, "session not found", nil)
	})
}

func TestAgentHandler_ListSessionHistories(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListSessionHistories
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		sessionUUID := "session-uuid-123"
		expectedHistories := []*types.AgentInstanceSessionHistory{
			{
				ID:          1,
				MsgUUID:     "msg-uuid-1",
				SessionID:   1,
				SessionUUID: sessionUUID,
				Request:     true,
				Content:     "Hello, how are you?",
				Feedback:    types.AgentSessionHistoryFeedbackNone,
				IsRewritten: false,
			},
			{
				ID:          2,
				MsgUUID:     "msg-uuid-2",
				SessionID:   1,
				SessionUUID: sessionUUID,
				Request:     false,
				Content:     "I'm doing well, thank you for asking!",
				Feedback:    types.AgentSessionHistoryFeedbackNone,
				IsRewritten: false,
			},
		}

		tester.mocks.agent.EXPECT().ListSessionHistories(tester.Ctx(), "u", sessionUUID, int64(16)).Return(expectedHistories, nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedHistories)
	})

	t.Run("session not found", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListSessionHistories
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "non-existent-session")

		tester.mocks.agent.EXPECT().ListSessionHistories(tester.Ctx(), "u", "non-existent-session", int64(16)).Return(nil, errors.New("session not found"))
		tester.Execute()

		tester.ResponseEq(t, 500, "session not found", nil)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListSessionHistories
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		tester.mocks.agent.EXPECT().ListSessionHistories(tester.Ctx(), "u", "session-uuid-123", int64(16)).Return(nil, errorx.ErrForbidden)
		tester.Execute()

		tester.ResponseEqCode(t, 403)
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListSessionHistories
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid").WithParam("session_uuid", "session-uuid-123")

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing session UUID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.ListSessionHistories
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "")

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})
}

func TestAgentHandler_CreateSessionHistory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSessionHistory
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		// Test with CreateSessionHistoryRequest containing messages
		content := `{"role": "user", "content": "Help me build an agent", "file": null, "timestamp": "2025-10-20T07:54:50.025Z"}`
		req := &types.CreateSessionHistoryRequest{
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content,
				},
			},
		}

		// The handler will set SessionUUID from the URL parameter
		expectedReq := &types.CreateSessionHistoryRequest{
			SessionUUID: "session-uuid-123",
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content,
				},
			},
		}

		expectedResponse := &types.CreateSessionHistoryResponse{
			MsgUUIDs: []string{"msg-uuid-1", "msg-uuid-2"},
		}

		tester.mocks.agent.EXPECT().CreateSessionHistories(tester.Ctx(), "u", int64(16), expectedReq).Return(expectedResponse, nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedResponse)
	})

	t.Run("multiple messages", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSessionHistory
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		content1 := `{"role": "user", "content": "Hello"}`
		content2 := `{"role": "assistant", "content": "Hi there"}`
		req := &types.CreateSessionHistoryRequest{
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content1,
				},
				{
					Request: false,
					Content: content2,
				},
			},
		}

		expectedReq := &types.CreateSessionHistoryRequest{
			SessionUUID: "session-uuid-123",
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content1,
				},
				{
					Request: false,
					Content: content2,
				},
			},
		}

		expectedResponse := &types.CreateSessionHistoryResponse{
			MsgUUIDs: []string{"msg-uuid-1", "msg-uuid-2"},
		}

		tester.mocks.agent.EXPECT().CreateSessionHistories(tester.Ctx(), "u", int64(16), expectedReq).Return(expectedResponse, nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedResponse)
	})

	t.Run("validation error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSessionHistory
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		// Invalid JSON
		tester.WithBody(t, "invalid json")
		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSessionHistory
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		content := `{"role": "user", "content": "Help me build an agent"}`
		req := &types.CreateSessionHistoryRequest{
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content,
				},
			},
		}

		expectedReq := &types.CreateSessionHistoryRequest{
			SessionUUID: "session-uuid-123",
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content,
				},
			},
		}

		tester.mocks.agent.EXPECT().CreateSessionHistories(tester.Ctx(), "u", int64(16), expectedReq).Return(nil, errors.New("database error"))
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSessionHistory
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123")

		content := `{"role": "user", "content": "Help me build an agent"}`
		req := &types.CreateSessionHistoryRequest{
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content,
				},
			},
		}

		expectedReq := &types.CreateSessionHistoryRequest{
			SessionUUID: "session-uuid-123",
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content,
				},
			},
		}

		tester.mocks.agent.EXPECT().CreateSessionHistories(tester.Ctx(), "u", int64(16), expectedReq).Return(nil, errorx.ErrForbidden)
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 403)
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSessionHistory
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid").WithParam("session_uuid", "session-uuid-123")

		req := &types.CreateSessionHistoryRequest{
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: "test",
				},
			},
		}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing session UUID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.CreateSessionHistory
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "")

		req := &types.CreateSessionHistoryRequest{
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: "test",
				},
			},
		}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})
}

func TestAgentHandler_UpdateSessionHistoryFeedback(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSessionHistoryFeedback
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "msg-uuid-123")

		req := &types.FeedbackSessionHistoryRequest{
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}

		expectedReq := &types.FeedbackSessionHistoryRequest{
			MsgUUID:  "msg-uuid-123",
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}

		tester.mocks.agent.EXPECT().UpdateSessionHistoryFeedback(tester.Ctx(), "u", int64(16), "session-uuid-123", expectedReq).Return(nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("validation error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSessionHistoryFeedback
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "msg-uuid-123")

		// Invalid JSON
		tester.WithBody(t, "invalid json")
		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSessionHistoryFeedback
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "msg-uuid-123")

		req := &types.FeedbackSessionHistoryRequest{
			Feedback: types.AgentSessionHistoryFeedbackDislike,
		}

		expectedReq := &types.FeedbackSessionHistoryRequest{
			MsgUUID:  "msg-uuid-123",
			Feedback: types.AgentSessionHistoryFeedbackDislike,
		}

		tester.mocks.agent.EXPECT().UpdateSessionHistoryFeedback(tester.Ctx(), "u", int64(16), "session-uuid-123", expectedReq).Return(errors.New("database error"))
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSessionHistoryFeedback
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "msg-uuid-123")

		req := &types.FeedbackSessionHistoryRequest{
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}

		expectedReq := &types.FeedbackSessionHistoryRequest{
			MsgUUID:  "msg-uuid-123",
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}

		tester.mocks.agent.EXPECT().UpdateSessionHistoryFeedback(tester.Ctx(), "u", int64(16), "session-uuid-123", expectedReq).Return(errorx.ErrForbidden)
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 403)
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSessionHistoryFeedback
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "msg-uuid-123")

		req := &types.FeedbackSessionHistoryRequest{
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing session UUID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSessionHistoryFeedback
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "").WithParam("msg_uuid", "msg-uuid-123")

		req := &types.FeedbackSessionHistoryRequest{
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing msg UUID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.UpdateSessionHistoryFeedback
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "")

		req := &types.FeedbackSessionHistoryRequest{
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})
}

func TestAgentHandler_RewriteMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.RewriteMessage
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "original-msg-uuid-123")

		req := &types.RewriteSessionHistoryRequest{
			Content: `{"role": "assistant", "content": "rewritten response"}`,
		}

		expectedReq := &types.RewriteSessionHistoryRequest{
			OriginalMsgUUID: "original-msg-uuid-123",
			Content:         `{"role": "assistant", "content": "rewritten response"}`,
		}

		expectedResponse := &types.RewriteSessionHistoryResponse{
			MsgUUID: "new-msg-uuid-123",
		}

		tester.mocks.agent.EXPECT().RewriteSessionHistory(tester.Ctx(), "u", int64(16), "session-uuid-123", expectedReq).Return(expectedResponse, nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, expectedResponse)
	})

	t.Run("validation error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.RewriteMessage
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "original-msg-uuid-123")

		// Invalid JSON
		tester.WithBody(t, "invalid json")
		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.RewriteMessage
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "original-msg-uuid-123")

		req := &types.RewriteSessionHistoryRequest{
			Content: "rewritten content",
		}

		expectedReq := &types.RewriteSessionHistoryRequest{
			OriginalMsgUUID: "original-msg-uuid-123",
			Content:         "rewritten content",
		}

		tester.mocks.agent.EXPECT().RewriteSessionHistory(tester.Ctx(), "u", int64(16), "session-uuid-123", expectedReq).Return(nil, errors.New("database error"))
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})

	t.Run("forbidden", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.RewriteMessage
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "original-msg-uuid-123")

		req := &types.RewriteSessionHistoryRequest{
			Content: "rewritten content",
		}

		expectedReq := &types.RewriteSessionHistoryRequest{
			OriginalMsgUUID: "original-msg-uuid-123",
			Content:         "rewritten content",
		}

		tester.mocks.agent.EXPECT().RewriteSessionHistory(tester.Ctx(), "u", int64(16), "session-uuid-123", expectedReq).Return(nil, errorx.ErrForbidden)
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 403)
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.RewriteMessage
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "invalid").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "original-msg-uuid-123")

		req := &types.RewriteSessionHistoryRequest{
			Content: "rewritten content",
		}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing session UUID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.RewriteMessage
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "").WithParam("msg_uuid", "original-msg-uuid-123")

		req := &types.RewriteSessionHistoryRequest{
			Content: "rewritten content",
		}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing msg UUID", func(t *testing.T) {
		tester := NewAgentServerHandlerTester(t).WithHandleFunc(func(h *AgentHandler) gin.HandlerFunc {
			return h.RewriteMessage
		})
		tester.WithKV("currentUserUUID", "u").WithParam("id", "16").WithParam("session_uuid", "session-uuid-123").WithParam("msg_uuid", "")

		req := &types.RewriteSessionHistoryRequest{
			Content: "rewritten content",
		}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})
}
