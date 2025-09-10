package checker

import (
	"context"
	"strings"
	"testing"

	"opencsg.com/csghub-server/common/types"
)

func TestGetFileChecker(t *testing.T) {
	// Test case 1: Test with a folder
	fileType1 := "folder"
	filePath1 := "/path/to/folder"
	lfsRelativePath1 := ""
	expected1 := &FolderChecker{}
	result1 := GetFileChecker(fileType1, filePath1, lfsRelativePath1)
	if _, ok := result1.(*FolderChecker); !ok {
		t.Errorf("Expected %T, got %T", expected1, result1)
	}

	// Test case 2: Test with a LFS file
	fileType2 := "file"
	filePath2 := "/path/to/lfs/file"
	lfsRelativePath2 := "lfs/path/to/file"
	expected2 := &LfsFileChecker{}
	result2 := GetFileChecker(fileType2, filePath2, lfsRelativePath2)
	if _, ok := result2.(*LfsFileChecker); !ok {
		t.Errorf("Expected %T, got %T", expected2, result2)
	}

	// Test case 3: Test with an unknown file type
	fileType3 := "file"
	filePath3 := "/path/to/unknown/file"
	lfsRelativePath3 := ""
	expected3 := &UnkownFileChecker{}
	result3 := GetFileChecker(fileType3, filePath3, lfsRelativePath3)
	if _, ok := result3.(*UnkownFileChecker); !ok {
		t.Errorf("Expected %T, got %T", expected3, result3)
	}

	// Test case 4: Test with an image file
	fileType4 := "file"
	filePath4 := "/path/to/image.png"
	lfsRelativePath4 := ""
	expected4 := &ImageFileChecker{}
	result4 := GetFileChecker(fileType4, filePath4, lfsRelativePath4)
	if _, ok := result4.(*ImageFileChecker); !ok {
		t.Errorf("Expected %T, got %T", expected4, result4)
	}

	// Test case 5: Test with a text file
	fileType5 := "file"
	filePath5 := "/path/to/text.md"
	lfsRelativePath5 := ""
	expected5 := &TextFileChecker{}
	result5 := GetFileChecker(fileType5, filePath5, lfsRelativePath5)
	if _, ok := result5.(*TextFileChecker); !ok {
		t.Errorf("Expected %T, got %T", expected5, result5)
	}
}

func TestImageFileChecker_Run(t *testing.T) {
	checker := &ImageFileChecker{}
	reader := strings.NewReader("image content")
	expectedStatus := types.SensitiveCheckSkip
	expectedMessage := "skip image file"
	status, message := checker.Run(context.Background(), reader)
	if status != expectedStatus || message != expectedMessage {
		t.Errorf("Expected (%v, %v), got (%v, %v)", expectedStatus, expectedMessage, status, message)
	}
}

func TestLfsFileChecker_Run(t *testing.T) {
	checker := &LfsFileChecker{}
	reader := strings.NewReader("lfs content")
	expectedStatus := types.SensitiveCheckSkip
	expectedMessage := "skip lfs file"
	status, message := checker.Run(context.Background(), reader)
	if status != expectedStatus || message != expectedMessage {
		t.Errorf("Expected (%v, %v), got (%v, %v)", expectedStatus, expectedMessage, status, message)
	}
}

func TestFolderChecker_Run(t *testing.T) {
	checker := &FolderChecker{}
	reader := strings.NewReader("folder content")
	expectedStatus := types.SensitiveCheckSkip
	expectedMessage := "skip folder"
	status, message := checker.Run(context.Background(), reader)
	if status != expectedStatus || message != expectedMessage {
		t.Errorf("Expected (%v, %v), got (%v, %v)", expectedStatus, expectedMessage, status, message)
	}
}
