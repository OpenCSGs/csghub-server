package errorx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createMockTestFile creates a temporary test file with various error handling patterns
func createMockTestFile(t *testing.T, content string) string {
	t.Helper()

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "error-scanner-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Cleanup function to delete temporary files after test completes
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Create test file
	filePath := filepath.Join(tempDir, "test.go")
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return filePath
}

// Mock Go file content for testing - contains various error handling patterns
var mockGoFileContent = `package main

import (
	"errors"
	"fmt"
)

func example() {
	// Test case 1: Meaningless errors.New() call
	err1 := errors.New("something")
	
	// Test case 2: Meaningful errors.New() call
	err2 := errors.New("invalid input error")
	
	// Test case 3: fmt.Errorf() without format placeholders
	err3 := fmt.Errorf("this is an error message")
	
	// Test case 4: Correct use of fmt.Errorf() with %w
	err4 := fmt.Errorf("failed to process: %w", err1)
	
	// Test case 5: errors.Wrap() with insufficient arguments
	// Only one argument passed, should be detected as an error
	// errors.Wrap(err1)
	
	// Test case 6: Correct errors.Wrap()
	// errors.Wrap(err1, "processing failed")
	
	// Test case 7: errors.Wrapf() with insufficient arguments
	// Only two arguments passed, should be detected as an error
	// errors.Wrapf(err1, "processing failed")
	
	// Test case 8: Correct errors.Wrapf()
	// errors.Wrapf(err1, "processing failed for %s", "user123")
	
	_ = err1
	_ = err2
	_ = err3
	_ = err4
}`

// TestScanFile tests whether scanFile function can correctly detect error patterns
func TestScanFile(t *testing.T) {
	// Create test file
	filePath := createMockTestFile(t, mockGoFileContent)

	// Execute scan
	results := scanFile(filePath)

	// Validate results
	if len(results) == 0 {
		t.Error("Expected to find issues, but found none")
	}

	// Count issues by type
	issueTypeCount := make(map[string]int)
	for _, result := range results {
		issueTypeCount[result.IssueType]++
	}

	// Verify detection of meaningless errors.New() calls
	if count, exists := issueTypeCount[IssueTypeErrorsNew]; !exists || count == 0 {
		t.Error("Expected to find issues with errors.New()")
	}

	// Verify detection of fmt.Errorf() calls without format placeholders
	if count, exists := issueTypeCount[IssueTypeFmtErrorf]; !exists || count == 0 {
		t.Error("Expected to find issues with fmt.Errorf()")
	}
}

// TestContainsErrorKeyword tests the containsErrorKeyword function
func TestContainsErrorKeyword(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"invalid input", true},
		{"error occurred", true},
		{"something happened", false},
		{"INVALID ERROR", true}, // Test case insensitivity
		{"missing value", true},
		{"regular message", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsErrorKeyword(tt.input)
			if result != tt.expected {
				t.Errorf("containsErrorKeyword(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestContainsFormatPlaceholder tests the containsFormatPlaceholder function
func TestContainsFormatPlaceholder(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"error: %w", true},
		{"value: %v", true},
		{"name: %s", true},
		{"count: %d", true},
		{"percentage: %f", true},
		{"plain text", false},
		{"text with %% sign", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsFormatPlaceholder(tt.input)
			if result != tt.expected {
				t.Errorf("containsFormatPlaceholder(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGetSuggestion tests the getSuggestion function
func TestGetSuggestion(t *testing.T) {
	tests := []struct {
		issueType        string
		expectedContains string
	}{
		{IssueTypeErrorsNew, "errorx.ErrForbidden"},
		{IssueTypeFmtErrorf, "%w"},
		{IssueTypeErrorsWrap, "errors.Wrap(err"},
		{IssueTypeErrorsWrapf, "errors.Wrapf(err"},
		{"unknown_type", "best practices"},
	}

	for _, tt := range tests {
		t.Run(tt.issueType, func(t *testing.T) {
			suggestion := getSuggestion(tt.issueType)
			if !strings.Contains(suggestion, tt.expectedContains) {
				t.Errorf("getSuggestion(%q) = %q; should contain %q", tt.issueType, suggestion, tt.expectedContains)
			}
		})
	}
}

// TestShouldSkipDir tests the shouldSkipDir function
func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		dir      string
		expected bool
	}{
		{"/path/to/.git", true},
		{"/path/to/vendor", true},
		{"/path/to/_mocks", true},
		{"/path/to/src", false},
		{"/path/to/components", false},
	}

	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			result := shouldSkipDir(tt.dir)
			if result != tt.expected {
				t.Errorf("shouldSkipDir(%q) = %v; want %v", tt.dir, result, tt.expected)
			}
		})
	}
}

// TestShouldSkipFile tests the shouldSkipFile function
func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		filePath string
		expected bool
	}{
		{"/path/to/file.go", false},
		{"/path/to/file_test.go", true},
		{"/path/to/file.js", true},
		{"/path/to/README.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := shouldSkipFile(tt.filePath)
			if result != tt.expected {
				t.Errorf("shouldSkipFile(%q) = %v; want %v", tt.filePath, result, tt.expected)
			}
		})
	}
}
