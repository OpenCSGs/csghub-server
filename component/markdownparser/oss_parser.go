package markdownparser

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// OSSImageInfo represents information about an Alibaba Cloud OSS object
type OSSImageInfo struct {
	BucketName string
	ObjectName string
	RegionId   string
}

// ParseResult represents the result of Markdown parsing
type ParseResult struct {
	Text             string
	OssImageInfoList []*OSSImageInfo
}

// OSSParserComponent defines the interface for OSS parsing component
type OSSParserComponent interface {
	// ParseMarkdownAndFilter parses Markdown content and extracts OSS image information
	ParseMarkdownAndFilter(markdownContent string) (*ParseResult, error)
	// ParseOSSUrl parses OSS URL and extracts information
	ParseOSSUrl(url string) (*OSSImageInfo, error)
	// IsWhitelistedImage checks if the image is in the whitelist
	IsWhitelistedImage(imgNode *ast.Image) bool
}

// ossParserComponentImpl implements OSSParserComponent interface
type ossParserComponentImpl struct {
	whitelistedExtensions map[string]bool
	ossUrlRegex           *regexp.Regexp
}

// NewOSSParserComponent creates a new instance of OSSParserComponent
func NewOSSParserComponent() OSSParserComponent {
	return &ossParserComponentImpl{
		whitelistedExtensions: map[string]bool{
			".png":  true,
			".jpg":  true,
			".jpeg": true,
			".bmp":  true,
			".webp": true,
			".tiff": true,
			".svg":  true,
			".heic": true,
			".gif":  true,
			".ico":  true,
		},
		ossUrlRegex: regexp.MustCompile(`^https?://([^.]+)\.oss-([^.]+)\.aliyuncs\.com/(.+)`),
	}
}

// ParseOSSUrl parses OSS URL and extracts information
func (o *ossParserComponentImpl) ParseOSSUrl(url string) (*OSSImageInfo, error) {
	matches := o.ossUrlRegex.FindStringSubmatch(url)
	if len(matches) != 4 {
		return nil, fmt.Errorf("URL '%s' format does not match Alibaba Cloud OSS standard format", url)
	}
	return &OSSImageInfo{
		BucketName: matches[1],
		RegionId:   matches[2],
		ObjectName: matches[3],
	}, nil
}

// IsWhitelistedImage checks if the image is in the whitelist
func (o *ossParserComponentImpl) IsWhitelistedImage(imgNode *ast.Image) bool {
	urlStr := string(imgNode.Destination)
	urlExt := strings.ToLower(path.Ext(urlStr))
	if o.whitelistedExtensions[urlExt] {
		return true
	}

	// OSS link without an extension, also regarded as a whitelist image
	if o.ossUrlRegex.MatchString(urlStr) && urlExt == "" {
		return true
	}

	var altText string
	if len(imgNode.Title) > 0 {
		altText = string(imgNode.Title)
	}
	altExt := strings.ToLower(path.Ext(altText))
	return o.whitelistedExtensions[altExt]
}

// ParseMarkdownAndFilter parses Markdown content and extracts OSS image information
func (o *ossParserComponentImpl) ParseMarkdownAndFilter(markdownContent string) (*ParseResult, error) {
	if markdownContent == "" {
		return nil, errors.New("input content cannot be empty")
	}

	source := []byte(markdownContent)
	p := goldmark.DefaultParser()
	root := p.Parse(text.NewReader(source))

	OssImageInfoList := []*OSSImageInfo{}

	// Record the image URL that needs to be removed from the text
	var removeImageUrls []string

	err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if img, isImage := node.(*ast.Image); isImage {
			urlStr := string(img.Destination)
			if o.IsWhitelistedImage(img) && o.ossUrlRegex.MatchString(urlStr) {
				if info, err := o.ParseOSSUrl(urlStr); err == nil {
					OssImageInfoList = append(OssImageInfoList, info)
					removeImageUrls = append(removeImageUrls, urlStr)
				}
			}
		}
		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk AST: %w", err)
	}

	// Regular expressions to remove Markdown syntax from images
	filteredText := markdownContent
	for _, imageUrl := range removeImageUrls {
		// Escape special characters for use in regular expressions
		escapedUrl := regexp.QuoteMeta(imageUrl)
		// Matches the Markdown syntax for images: ![alt text](url) or ![alt text](url "title")
		imagePattern := fmt.Sprintf(`!\[[^\]]*\]\(%s(?:\s+"[^"]*")?\)`, escapedUrl)
		imageRegex := regexp.MustCompile(imagePattern)
		filteredText = imageRegex.ReplaceAllString(filteredText, "")
	}

	// Clean up extra blank lines
	filteredText = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(filteredText, "\n")
	filteredText = strings.TrimSpace(filteredText)

	return &ParseResult{
		Text:             filteredText,
		OssImageInfoList: OssImageInfoList,
	}, nil
}
