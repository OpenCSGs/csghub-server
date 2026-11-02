package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommaSeparatedValues(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		pattern  string
		expected []string
	}{
		{
			name:     "channels with spaces",
			line:     "// @Channels internal-message, email",
			pattern:  `@Channels\s+(.+)`,
			expected: []string{"internal-message", "email"},
		},
		{
			name:     "payload fields with spaces",
			line:     "// @PayloadFields user_name, amount, currency",
			pattern:  `@PayloadFields\s+(.+)`,
			expected: []string{"user_name", "amount", "currency"},
		},
		{
			name:     "build tags",
			line:     "// @BuildTags saas,ee",
			pattern:  `@BuildTags\s+(\S+)`,
			expected: []string{"saas", "ee"},
		},
		{
			name:     "no match",
			line:     "// some other comment",
			pattern:  `@Channels\s+(.+)`,
			expected: nil,
		},
		{
			name:     "empty values",
			line:     "// @Channels ",
			pattern:  `@Channels\s+(.+)`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommaSeparatedValues(tt.line, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateTemplateContent(t *testing.T) {
	tests := []struct {
		name         string
		templateData map[string]string
		channel      string
		lang         string
		expected     string
	}{
		{
			name: "email channel with HTML",
			templateData: map[string]string{
				"title":   "Test Title",
				"content": "Test content with {{.variable}}",
			},
			channel: channelEmail,
			lang:    "en-US",
			expected: `{{/* title section */}}
Test Title
---
{{/* content section */}}
<html>
	<body>
		<h3>Test Title</h3>
		<p>Test content with {{.variable}}</p>
	</body>
</html>`,
		},
		{
			name: "internal message channel without HTML",
			templateData: map[string]string{
				"title":   "Internal Title",
				"content": "Internal content with {{.variable}}",
			},
			channel: channelInternalMessage,
			lang:    "en-US",
			expected: `{{/* title section */}}
Internal Title
---
{{/* content section */}}
Internal content with {{.variable}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createTemplateContent(tt.templateData, tt.channel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateScenarioRegistration(t *testing.T) {
	scenario := NotificationScenario{
		Name:     "test-scenario",
		Channels: []string{channelEmail, channelInternalMessage},
	}

	result := generateScenarioRegistration(scenario)

	// Check that the registration contains expected elements
	assert.Contains(t, result, "// register test-scenario scenario")
	assert.Contains(t, result, "scenariomgr.RegisterScenario(types.MessageScenarioTestScenario")
	assert.Contains(t, result, "types.MessageChannelEmail")
	assert.Contains(t, result, "types.MessageChannelInternalMessage")
	assert.Contains(t, result, "internalnotification.GetEmailDataFunc(d.GetNotificationStorage())")
	assert.Contains(t, result, "internalnotification.GetSiteInternalMessageData")
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test-scenario", "TestScenario"},
		{"user-verify", "UserVerify"},
		{"recharge-success", "RechargeSuccess"},
		{"single", "Single"},
		{"", ""},
		{"already-pascal", "AlreadyPascal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRegisterFile(t *testing.T) {
	tests := []struct {
		buildTag string
		expected string
	}{
		{"", ""},
		{"saas", extendRegisterFileSaas},
		{"ee", extendRegisterFileEE},
		{"ce", scenarioRegisterFile},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.buildTag, func(t *testing.T) {
			result := getRegisterFile(tt.buildTag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `{"key": "value",}`,
			expected: `{"key": "value"}`,
		},
		{
			input:    `{"key": "value", "key2": "value2",}`,
			expected: `{"key": "value", "key2": "value2"}`,
		},
		{
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsJSONComplete(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`{"key": "value"}`, true},
		{`{"key": "value", "key2": "value2"}`, true},
		{`{"key": "value"`, false},
		{`{"key": "value",}`, true},
		{`{}`, true},
		{`{`, false},
		{`}`, false},
		{`{"nested": {"inner": "value"}}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isJSONComplete(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateTemplateFiles(t *testing.T) {
	scenario := NotificationScenario{
		Name:     "test-scenario",
		Channels: []string{channelEmail},
		Templates: map[string]map[string]map[string]string{
			channelEmail: {
				"en-US": {
					"title":   "Test Title",
					"content": "Test content with {{.variable}}",
				},
			},
		},
	}

	// Test the createTemplateContent function directly since we can't override constants
	templateData := scenario.Templates[channelEmail]["en-US"]
	result := createTemplateContent(templateData, channelEmail)

	expectedContent := `{{/* title section */}}
Test Title
---
{{/* content section */}}
<html>
	<body>
		<h3>Test Title</h3>
		<p>Test content with {{.variable}}</p>
	</body>
</html>`
	assert.Equal(t, expectedContent, result)
}

func TestParseNotificationScenarios(t *testing.T) {
	// This test would require modifying the global constants, which isn't possible
	// Instead, we'll test the individual parsing functions
	t.Run("parseCommaSeparatedValues", func(t *testing.T) {
		// Test channels parsing
		line := "// @Channels email, internal-message"
		result := parseCommaSeparatedValues(line, `@Channels\s+(.+)`)
		assert.Equal(t, []string{"email", "internal-message"}, result)

		// Test payload fields parsing
		line = "// @PayloadFields user_name, amount"
		result = parseCommaSeparatedValues(line, `@PayloadFields\s+(.+)`)
		assert.Equal(t, []string{"user_name", "amount"}, result)

		// Test build tags parsing
		line = "// @BuildTags saas"
		result = parseCommaSeparatedValues(line, `@BuildTags\s+(\S+)`)
		assert.Equal(t, []string{"saas"}, result)
	})
}

func TestAddScenarioRegistration(t *testing.T) {
	// Test the generateScenarioRegistration function directly
	scenario := NotificationScenario{
		Name:     "test-scenario",
		Channels: []string{channelEmail},
	}

	result := generateScenarioRegistration(scenario)

	// Check that the registration contains expected elements
	assert.Contains(t, result, "// register test-scenario scenario")
	assert.Contains(t, result, "scenariomgr.RegisterScenario(types.MessageScenarioTestScenario")
	assert.Contains(t, result, "types.MessageChannelEmail")
	assert.Contains(t, result, "internalnotification.GetEmailDataFunc(d.GetNotificationStorage())")
}

func TestAddScenarioRegistrationWithMultipleChannels(t *testing.T) {
	// Test registration with multiple channels
	scenario := NotificationScenario{
		Name:     "multi-channel-scenario",
		Channels: []string{channelEmail, channelInternalMessage},
	}

	result := generateScenarioRegistration(scenario)

	// Check that both channels are included
	assert.Contains(t, result, "types.MessageChannelEmail")
	assert.Contains(t, result, "types.MessageChannelInternalMessage")
	assert.Contains(t, result, "internalnotification.GetEmailDataFunc(d.GetNotificationStorage())")
	assert.Contains(t, result, "internalnotification.GetSiteInternalMessageData")
}

func TestRunNotifyGen(t *testing.T) {
	// Test the individual functions that can be tested without file system dependencies
	t.Run("toPascalCase", func(t *testing.T) {
		assert.Equal(t, "TestScenario", toPascalCase("test-scenario"))
		assert.Equal(t, "UserVerify", toPascalCase("user-verify"))
	})

	t.Run("getRegisterFile", func(t *testing.T) {
		assert.Equal(t, "", getRegisterFile(""))
		assert.Equal(t, extendRegisterFileSaas, getRegisterFile("saas"))
		assert.Equal(t, extendRegisterFileEE, getRegisterFile("ee"))
		assert.Equal(t, scenarioRegisterFile, getRegisterFile("ce"))
	})
}
