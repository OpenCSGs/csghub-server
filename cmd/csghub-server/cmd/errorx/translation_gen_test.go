package errorx

import (
	"fmt"
	"strings"
	"testing"
)

func TestGenerateMarkdownDoc(t *testing.T) {
	// --- Test Data Setup ---
	sampleInfos := map[string][]ErrorInfo{
		// This file should appear second in the output due to alphabetical sorting.
		"error_user.go": {
			{
				Code:           101,
				ConstName:      "ErrUserNotFound",
				Description:    "The requested user was not found.",
				Description_ZH: "未找到请求的用户。",
				FullCode:       "USER-101",
				Translations:   map[string]string{"en-US": "User not found."},
			},
			{
				Code:           102,
				ConstName:      "ErrUserInvalid",
				Description:    "User data for {userID} is invalid.", // Contains a placeholder
				Description_ZH: "用户{userID}的数据无效。",
				FullCode:       "USER-102",
				Translations:   map[string]string{"en-US": "User data for {userID} is invalid."},
			},
		},
		// This file should appear first.
		"error_auth.go": {
			{
				Code:           201,
				ConstName:      "ErrAuthTokenExpired",
				Description:    "Authentication token has expired.",
				Description_ZH: "", // Empty Chinese description to test fallback
				FullCode:       "AUTH-201",
				Translations:   map[string]string{"en-US": "Token expired."},
			},
		},
		"error_empty.go": {}, // This file should be ignored in the output.
	}

	// --- Test Cases ---
	tests := []struct {
		name           string
		infosByFile    map[string][]ErrorInfo
		config         MarkdownConfig
		expectedOutput string
	}{
		{
			name:        "English Documentation Generation",
			infosByFile: sampleInfos,
			config: MarkdownConfig{
				Title:         "# Error Codes",
				IntroText:     "This is a list of error codes.",
				ChapterFormat: "## %s Errors",
				DetailLabels: map[string]string{
					"FullCode":     "Error Code",
					"ConstantName": "Constant Name",
					"Description":  "Description",
				},
				Lang: "en",
			},
			expectedOutput: buildExpectedOutput(
				"# Error Codes\n\n",
				"This is a list of error codes.\n\n",
				// Chapter 1: Auth
				"## Auth Errors\n\n",
				"### `AUTH-201`\n\n",
				"- **Error Code:** `AUTH-201`\n",
				"- **Constant Name:** `ErrAuthTokenExpired`\n",
				"- **Description:** Authentication token has expired.\n\n",
				// Chapter 2: User
				"## User Errors\n\n",
				"### `USER-101`\n\n",
				"- **Error Code:** `USER-101`\n",
				"- **Constant Name:** `ErrUserNotFound`\n",
				"- **Description:** The requested user was not found.\n",
				"\n---\n\n",
				"### `USER-102`\n\n",
				"- **Error Code:** `USER-102`\n",
				"- **Constant Name:** `ErrUserInvalid`\n",
				"- **Description:** User data for `{userID}` is invalid.\n\n", // Placeholder should be formatted
			),
		},
		{
			name:        "Chinese Documentation Generation with Fallback",
			infosByFile: sampleInfos,
			config: MarkdownConfig{
				Title:         "# 错误代码",
				IntroText:     "这是一个错误代码列表。",
				ChapterFormat: "## %s 错误",
				DetailLabels: map[string]string{
					"FullCode":     "错误代码",
					"ConstantName": "常量名",
					"Description":  "描述",
				},
				Lang: "zh",
			},
			expectedOutput: buildExpectedOutput(
				"# 错误代码\n\n",
				"这是一个错误代码列表。\n\n",
				// Chapter 1: Auth
				"## Auth 错误\n\n",
				"### `AUTH-201`\n\n",
				"- **错误代码:** `AUTH-201`\n",
				"- **常量名:** `ErrAuthTokenExpired`\n",
				"- **描述:** Authentication token has expired.\n\n", // Fallback to English description
				// Chapter 2: User
				"## User 错误\n\n",
				"### `USER-101`\n\n",
				"- **错误代码:** `USER-101`\n",
				"- **常量名:** `ErrUserNotFound`\n",
				"- **描述:** 未找到请求的用户。\n", // Using Chinese description
				"\n---\n\n",
				"### `USER-102`\n\n",
				"- **错误代码:** `USER-102`\n",
				"- **常量名:** `ErrUserInvalid`\n",
				"- **描述:** 用户`{userID}`的数据无效。\n\n", // Chinese description with formatted placeholder
			),
		},
		{
			name:        "Empty Input Data",
			infosByFile: map[string][]ErrorInfo{},
			config: MarkdownConfig{
				Title:     "# Empty Test",
				IntroText: "No errors to show.",
				Lang:      "en",
			},
			expectedOutput: "# Empty Test\n\nNo errors to show.\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := generateMarkdownDoc(tt.infosByFile, tt.config)
			if err != nil {
				t.Fatalf("generateMarkdownDoc() returned an unexpected error: %v", err)
			}

			got := string(gotBytes)
			// Normalize line endings for consistent comparison across platforms
			got = strings.ReplaceAll(got, "\r\n", "\n")
			expected := strings.ReplaceAll(tt.expectedOutput, "\r\n", "\n")

			if got != expected {
				t.Errorf("generateMarkdownDoc() output mismatch.\n--- GOT ---\n%s\n\n--- WANT ---\n%s", got, expected)
				// For easier debugging, print a diff-like view
				fmt.Println("--- DIFF ---")
				printDiff(t, got, expected)
			}
		})
	}
}

// buildExpectedOutput is a helper to concatenate strings for the expected result.
func buildExpectedOutput(parts ...string) string {
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p)
	}
	return sb.String()
}

// printDiff helps visualize the difference between two strings.
func printDiff(t *testing.T, got, want string) {
	// A simple line-by-line comparison for demonstration
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")
	maxLines := len(gotLines)
	if len(wantLines) > maxLines {
		maxLines = len(wantLines)
	}

	for i := 0; i < maxLines; i++ {
		var gLine, wLine string
		if i < len(gotLines) {
			gLine = gotLines[i]
		}
		if i < len(wantLines) {
			wLine = wantLines[i]
		}

		if gLine != wLine {
			t.Logf("Line %d mismatch:", i+1)
			t.Logf(" GOT: %q", gLine)
			t.Logf("WANT: %q", wLine)
		}
	}
}
