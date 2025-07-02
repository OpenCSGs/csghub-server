package markdownparser

import (
	"bytes"
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
	IsWhitelistedImage(imgNode *ast.Image, source []byte) bool
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
func (o *ossParserComponentImpl) IsWhitelistedImage(imgNode *ast.Image, source []byte) bool {
	// 1. Try to get extension from URL
	urlExt := strings.ToLower(path.Ext(string(imgNode.Destination)))
	if o.whitelistedExtensions[urlExt] {
		return true
	}

	// 2. If URL doesn't have a valid extension, try to get from alt text
	// Directly get the text content of the Image node
	var altText string
	if len(imgNode.Text(source)) > 0 {
		altText = string(imgNode.Text(source))
	} else {
		// If Text method returns empty, try to use other ways to get alt text
		// This may need to be adjusted according to the implementation of the goldmark library
		altText = string(imgNode.Title)
	}

	altExt := strings.ToLower(path.Ext(altText))
	if o.whitelistedExtensions[altExt] {
		return true
	}

	return false
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
	var finalResultBuilder strings.Builder

	for blockNode := root.FirstChild(); blockNode != nil; blockNode = blockNode.NextSibling() {
		var blockContentBuilder bytes.Buffer

		for inlineNode := blockNode.FirstChild(); inlineNode != nil; inlineNode = inlineNode.NextSibling() {
			img, isImage := inlineNode.(*ast.Image)
			if !isImage {
				blockContentBuilder.Write(inlineNode.Text(source))
				continue
			}

			// Process image node
			blockContentBuilder.Write(img.Text(source))

			// Check if it's a whitelisted image
			if o.IsWhitelistedImage(img, source) {
				// Check if it's an OSS image
				if o.ossUrlRegex.MatchString(string(img.Destination)) {
					if info, err := o.ParseOSSUrl(string(img.Destination)); err == nil {
						OssImageInfoList = append(OssImageInfoList, info)
					}
				}
			}
		}

		finalResultBuilder.Write(blockContentBuilder.Bytes())

		if next := blockNode.NextSibling(); next != nil {
			startOfNext := next.Lines().At(0).Start
			endOfCurrent := blockNode.Lines().At(blockNode.Lines().Len() - 1).Stop
			if startOfNext > endOfCurrent {
				finalResultBuilder.Write(source[endOfCurrent:startOfNext])
			}
		}
	}

	return &ParseResult{
		Text:             finalResultBuilder.String(),
		OssImageInfoList: OssImageInfoList,
	}, nil
}
