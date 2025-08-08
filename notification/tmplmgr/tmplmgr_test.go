package tmplmgr

import (
	"bytes"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestNewTemplateManager(t *testing.T) {
	tm := NewTemplateManager()
	assert.NotNil(t, tm)
}

func TestTemplateManager_ParseTemplateOutput(t *testing.T) {
	tm := NewTemplateManager()

	testCases := []struct {
		name            string
		input           string
		expectedTitle   string
		expectedContent string
	}{
		{
			name:            "valid title and content with separator",
			input:           "Test Title---Test Content",
			expectedTitle:   "Test Title",
			expectedContent: "Test Content",
		},
		{
			name:            "title and content with whitespace",
			input:           "  Test Title  ---  Test Content  ",
			expectedTitle:   "Test Title",
			expectedContent: "Test Content",
		},
		{
			name:            "no separator - entire output as content",
			input:           "Just some content without separator",
			expectedTitle:   "",
			expectedContent: "Just some content without separator",
		},
		{
			name:            "empty input",
			input:           "",
			expectedTitle:   "",
			expectedContent: "",
		},
		{
			name:            "only title with separator",
			input:           "Test Title---",
			expectedTitle:   "Test Title",
			expectedContent: "",
		},
		{
			name:            "only content with separator",
			input:           "---Test Content",
			expectedTitle:   "",
			expectedContent: "Test Content",
		},
		{
			name:            "multiple separators - only first split",
			input:           "Title---Content---Extra",
			expectedTitle:   "Title",
			expectedContent: "Content---Extra",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tm.parseTemplateOutput(tc.input)

			assert.Equal(t, tc.expectedTitle, result.Title)
			assert.Equal(t, tc.expectedContent, result.Content)
		})
	}
}

func TestTemplateManager_Format_DefaultEmailTemplate(t *testing.T) {
	tm := NewTemplateManager()

	data := struct {
		Title   string
		Content string
	}{
		Title:   "Test Title",
		Content: "Test Content",
	}

	// Test with a scenario that doesn't exist, should fall back to default
	result, err := tm.Format("non-existent-scenario", types.MessageChannelEmail, data, "en-US")
	require.NoError(t, err)

	// Should contain the default email template structure
	assert.Contains(t, result.Content, "<html>")
	assert.Contains(t, result.Content, "<p>")
	assert.Contains(t, result.Content, "Test Title")
	assert.Contains(t, result.Content, "Test Content")
}

func TestTemplateManager_Format_InternalNotificationEmailTemplate(t *testing.T) {
	tm := NewTemplateManager()

	data := map[string]any{
		"title":   "Test Title",
		"summary": "Test Summary",
		"content": "Test Content",
	}

	// Test with internal-notification scenario
	result, err := tm.Format(types.MessageScenarioInternalNotification, types.MessageChannelEmail, data, "en-US")
	require.NoError(t, err)

	// Should contain the internal-notification email template structure
	assert.Contains(t, result.Content, "<html>")
	assert.Contains(t, result.Content, "<h3>")
	assert.Contains(t, result.Content, "<span>")
	assert.Contains(t, result.Content, "Test Title")
	assert.Contains(t, result.Content, "Test Summary")
	assert.Contains(t, result.Content, "Test Content")
}

func TestTemplateManager_Format_CacheBehavior(t *testing.T) {
	tm := NewTemplateManager()

	data := struct {
		Title   string
		Content string
	}{
		Title:   "Cache Test",
		Content: "Cache Content",
	}

	// First call - should load from embedded templates
	result1, err := tm.Format("non-existent-scenario", types.MessageChannelEmail, data, "en-US")
	require.NoError(t, err)
	assert.Contains(t, result1.Content, "Cache Test")

	// Second call - should use cached template
	result2, err := tm.Format("non-existent-scenario", types.MessageChannelEmail, data, "en-US")
	require.NoError(t, err)
	assert.Contains(t, result2.Content, "Cache Test")

	// Results should be identical
	assert.Equal(t, result1, result2)
}

func TestTemplateManager_Format_ComplexDataStructure(t *testing.T) {
	tm := NewTemplateManager()

	data := map[string]any{
		"title":     "Complex Test",
		"summary":   "Complex Summary",
		"content":   "Complex Content",
		"timestamp": time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		"count":     42,
		"enabled":   true,
	}

	result, err := tm.Format(types.MessageScenarioInternalNotification, types.MessageChannelEmail, data, "en-US")
	require.NoError(t, err)

	assert.Contains(t, result.Content, "Complex Test")
	assert.Contains(t, result.Content, "Complex Summary")
	assert.Contains(t, result.Content, "Complex Content")
}

func TestTemplateManager_Format_InvalidChannel(t *testing.T) {
	tm := NewTemplateManager()

	data := struct {
		Title   string
		Content string
	}{
		Title:   "Test",
		Content: "Test",
	}

	// Test with an invalid channel that doesn't have a default template
	result, err := tm.Format("non-existent-scenario", "invalid-channel", data, "en-US")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default template file not found")
	assert.Empty(t, result)
}

func TestTemplateManager_Format_ConcurrentAccess(t *testing.T) {
	tm := NewTemplateManager()

	data := struct {
		Title   string
		Content string
	}{
		Title:   "Concurrent Test",
		Content: "Concurrent Content",
	}

	// Test concurrent access to the same template
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			result, err := tm.Format("non-existent-scenario", types.MessageChannelEmail, data, "en-US")
			assert.NoError(t, err)
			assert.Contains(t, result.Content, "Concurrent Test")
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestTemplateManager_ExecuteTemplate_Error(t *testing.T) {
	tm := NewTemplateManager()

	// Create a template that will fail to execute
	tmpl, err := template.New("test").Parse("{{.NonExistentField}}")
	require.NoError(t, err)

	data := struct {
		Title string
	}{
		Title: "Test",
	}

	result, err := tm.executeTemplate(tmpl, data, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute template")
	assert.Empty(t, result)
}

func TestTemplateManager_LoadDefaultTemplate_NonExistent(t *testing.T) {
	tm := NewTemplateManager()

	// Test loading a default template that doesn't exist
	tmpl, err := tm.loadDefaultTemplate(types.MessageChannelInternalMessage, "en-US")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default template file not found")
	assert.Nil(t, tmpl)
}

func TestTemplateManager_LoadDefaultTemplate_Valid(t *testing.T) {
	tm := NewTemplateManager()

	// Test loading a valid default template
	tmpl, err := tm.loadDefaultTemplate(types.MessageChannelEmail, "en-US")
	assert.NoError(t, err)
	assert.NotNil(t, tmpl)

	// Test that the template can be executed
	data := struct {
		Title   string
		Content string
	}{
		Title:   "Test Title",
		Content: "Test Content",
	}

	// Convert struct to map since the default template expects map data for iteration
	templateData := tm.normalizeTemplateData(data)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	assert.NoError(t, err)
	result := buf.String()

	// Should contain the default email template structure
	assert.Contains(t, result, "<html>")
	assert.Contains(t, result, "Notification")
	// Should contain each field in its own paragraph with "key: value" format
	assert.Contains(t, result, "<p>Title: Test Title</p>")
	assert.Contains(t, result, "<p>Content: Test Content</p>")
}

func TestTemplateManager_Format_DifferentChannels(t *testing.T) {
	tm := NewTemplateManager()

	data := struct {
		Title   string
		Content string
	}{
		Title:   "Channel Test",
		Content: "Channel Content",
	}

	// Test different channels with the same scenario
	channels := []types.MessageChannel{
		types.MessageChannelEmail,
	}

	for _, channel := range channels {
		result, err := tm.Format("non-existent-scenario", channel, data, "en-US")
		require.NoError(t, err)
		assert.Contains(t, result.Content, "Channel Test")
		assert.Contains(t, result.Content, "Channel Content")
	}
}

func TestTemplateManager_Format_MemoryEfficiency(t *testing.T) {
	tm := NewTemplateManager()

	data := struct {
		Title   string
		Content string
	}{
		Title:   "Memory Test",
		Content: "Memory Content",
	}

	// Call the same template multiple times
	for i := 0; i < 100; i++ {
		result, err := tm.Format("non-existent-scenario", types.MessageChannelEmail, data, "en-US")
		require.NoError(t, err)
		assert.Contains(t, result.Content, "Memory Test")
	}

	// The cache should prevent repeated template parsing
	// We can't easily test memory usage in unit tests, but we can verify the cache is working
	// by checking that subsequent calls don't fail
}

func TestTemplateManager_Format_AnyDataWithDefaultTemplate(t *testing.T) {
	tm := NewTemplateManager()

	// Test with any data structure
	data := struct {
		Title   string
		Message string
		Count   int
	}{
		Title:   "Test Title",
		Message: "This is a test message",
		Count:   42,
	}

	// Test with default email template (non-existent scenario)
	result, err := tm.Format("non-existent-scenario", types.MessageChannelEmail, data, "en-US")
	require.NoError(t, err)

	// Should contain the default email template structure
	assert.Contains(t, result.Content, "<html>")
	assert.Contains(t, result.Content, "Notification")
	// Should contain each field in its own paragraph
	assert.Contains(t, result.Content, "<p>Title: Test Title</p>")
	assert.Contains(t, result.Content, "<p>Message: This is a test message</p>")
	assert.Contains(t, result.Content, "<p>Count: 42</p>")
}

func TestTemplateManager_Format_ScenarioSpecificTemplateUsesOriginalData(t *testing.T) {
	tm := NewTemplateManager()

	// Test with data that has Title, Summary, and Content fields
	data := map[string]any{
		"title":   "Test Title",
		"summary": "Test Summary",
		"content": "Test Content",
	}

	// Test with internal-notification scenario (which has a specific template)
	result, err := tm.Format(types.MessageScenarioInternalNotification, types.MessageChannelEmail, data, "en-US")
	require.NoError(t, err)

	// Should contain the scenario-specific template structure
	assert.Contains(t, result.Content, "<html>")
	assert.Contains(t, result.Content, "<h3>")
	assert.Contains(t, result.Content, "<span>")
	assert.Contains(t, result.Content, "Test Title")
	assert.Contains(t, result.Content, "Test Summary")
	assert.Contains(t, result.Content, "Test Content")

	// Should NOT contain the default template's "Notification" text
	assert.NotContains(t, result.Content, "Notification")
}

func TestTemplateManager_ConvertStructToMap(t *testing.T) {
	tm := NewTemplateManager()

	// Test with struct data
	data := struct {
		Name   string
		Age    int
		Active bool
		Score  float64
	}{
		Name:   "John Doe",
		Age:    30,
		Active: true,
		Score:  95.5,
	}

	result := tm.normalizeTemplateData(data)

	// Should contain all struct fields as map keys
	assert.Equal(t, "John Doe", result["Name"])
	assert.Equal(t, 30, result["Age"])
	assert.Equal(t, true, result["Active"])
	assert.Equal(t, 95.5, result["Score"])
	assert.Len(t, result, 4)
}

func TestTemplateManager_ConvertStructToMap_NonStruct(t *testing.T) {
	tm := NewTemplateManager()

	// Test with non-struct data
	testCases := []struct {
		name     string
		data     any
		expected any
	}{
		{"string", "hello world", "hello world"},
		{"int", 42, 42},
		{"bool", true, true},
		{"slice", []string{"a", "b"}, []string{"a", "b"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tm.normalizeTemplateData(tc.data)
			assert.Equal(t, tc.expected, result["Content"])
			assert.Len(t, result, 1)
		})
	}
}

func TestTemplateManager_Format_AssetManagementEmailTemplate_DifferentLanguages(t *testing.T) {
	tm := NewTemplateManager()

	testCases := []struct {
		language        string
		expectedTitle   string
		expectedContent string
	}{
		{
			language:        "en-US",
			expectedTitle:   "[model] Created",
			expectedContent: "[testuser/awesome-model] created successfully.",
		},
		{
			language:        "zh-CN",
			expectedTitle:   "[model] 已创建",
			expectedContent: "[testuser/awesome-model] 创建成功。",
		},
		{
			language:        "zh-HK",
			expectedTitle:   "[model] 已創建",
			expectedContent: "[testuser/awesome-model] 創建成功。",
		},
	}

	data := map[string]any{
		"repo_type": types.ModelRepo,
		"repo_path": "testuser/awesome-model",
		"operation": types.OperationCreate,
	}

	for _, tc := range testCases {
		t.Run("email_language_"+tc.language, func(t *testing.T) {
			result, err := tm.Format(types.MessageScenarioAssetManagement, types.MessageChannelEmail, data, tc.language)
			require.NoError(t, err)

			// Should contain the localized title in h3 tag
			assert.Equal(t, result.Title, tc.expectedTitle)

			// Should contain the localized content in span tag
			assert.Contains(t, result.Content, tc.expectedContent)

			// Should contain the basic HTML structure
			assert.Contains(t, result.Content, "<html>")
			assert.Contains(t, result.Content, "<p>")
		})
	}
}
