package markdownparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yuin/goldmark/ast"
)

func TestNewOSSParserComponent(t *testing.T) {
	parser := NewOSSParserComponent()
	assert.NotNil(t, parser, "Parser should not be nil")

	// Check if it's the correct implementation type
	_, ok := parser.(*ossParserComponentImpl)
	assert.True(t, ok, "Parser should be of type ossParserComponentImpl")
}

func TestParseOSSUrl(t *testing.T) {
	parser := NewOSSParserComponent()

	tests := []struct {
		name        string
		url         string
		expectError bool
		expected    *OSSImageInfo
	}{
		{
			name:        "Valid OSS URL",
			url:         "https://mybucket.oss-cn-beijing.aliyuncs.com/path/to/image.jpg",
			expectError: false,
			expected: &OSSImageInfo{
				BucketName: "mybucket",
				RegionId:   "cn-beijing",
				ObjectName: "path/to/image.jpg",
			},
		},
		{
			name:        "Invalid OSS URL",
			url:         "https://example.com/image.jpg",
			expectError: true,
			expected:    nil,
		},
		{
			name:        "HTTP OSS URL",
			url:         "http://testbucket.oss-cn-shanghai.aliyuncs.com/folder/image.png",
			expectError: false,
			expected: &OSSImageInfo{
				BucketName: "testbucket",
				RegionId:   "cn-shanghai",
				ObjectName: "folder/image.png",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseOSSUrl(tt.url)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.BucketName, result.BucketName)
				assert.Equal(t, tt.expected.RegionId, result.RegionId)
				assert.Equal(t, tt.expected.ObjectName, result.ObjectName)
			}
		})
	}
}

func TestIsWhitelistedImage(t *testing.T) {
	parser := NewOSSParserComponent().(*ossParserComponentImpl)

	// Create a mock image node with a whitelisted extension
	imgWithValidExt := &ast.Image{}
	imgWithValidExt.Destination = []byte("https://example.com/image.jpg")

	// Create a mock image node without a valid extension but with alt text having extension
	imgWithAltExt := &ast.Image{}
	imgWithAltExt.Destination = []byte("https://example.com/image")
	// 正确设置 alt text - 使用 Title 属性代替 SetText
	imgWithAltExt.Title = []byte("alt-text.png")
	source := []byte("content with alt-text.png")

	// Create a mock image node without any valid extension
	imgWithoutExt := &ast.Image{}
	imgWithoutExt.Destination = []byte("https://example.com/image")

	tests := []struct {
		name     string
		img      *ast.Image
		source   []byte
		expected bool
	}{
		{
			name:     "Image with valid extension in URL",
			img:      imgWithValidExt,
			source:   []byte{},
			expected: true,
		},
		{
			name:     "Image with valid extension in alt text",
			img:      imgWithAltExt,
			source:   source,
			expected: true,
		},
		{
			name:     "Image without valid extension",
			img:      imgWithoutExt,
			source:   []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsWhitelistedImage(tt.img, tt.source)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMarkdownAndFilter(t *testing.T) {
	parser := NewOSSParserComponent()

	tests := []struct {
		name           string
		markdown       string
		expectError    bool
		expectedOssLen int
	}{
		{
			name:           "Empty markdown",
			markdown:       "",
			expectError:    true,
			expectedOssLen: 0,
		},
		{
			name:           "Markdown without images",
			markdown:       "# Test Heading\nThis is a test paragraph without any images.",
			expectError:    false,
			expectedOssLen: 0,
		},
		{
			name:           "Markdown with non-OSS image",
			markdown:       "# Test\n![alt.jpg](https://example.com/image.jpg)",
			expectError:    false,
			expectedOssLen: 0,
		},
		{
			name:           "Markdown with OSS image",
			markdown:       "# Test\n![alt.jpg](https://mybucket.oss-cn-beijing.aliyuncs.com/xxxx/id)",
			expectError:    false,
			expectedOssLen: 1,
		},
		{
			name:           "Markdown with multiple OSS images",
			markdown:       "# Test\n![alt1.jpg](https://bucket1.oss-cn-beijing.aliyuncs.com/xxxx/id)\n![alt2.png](https://bucket2.oss-cn-shanghai.aliyuncs.com/yyyy/id)",
			expectError:    false,
			expectedOssLen: 2,
		},
		{
			name:           "Markdown with mixed images",
			markdown:       "# Test\n![alt1.jpg](https://example.com/image.jpg)\n![alt2.jpg](https://bucket.oss-cn-beijing.aliyuncs.com/zzzz/id)",
			expectError:    false,
			expectedOssLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseMarkdownAndFilter(tt.markdown)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result.OssImageInfoList, tt.expectedOssLen)
			}
		})
	}
}
