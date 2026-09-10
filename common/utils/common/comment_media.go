package common

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	markdownast "github.com/yuin/goldmark/ast"
	markdowntext "github.com/yuin/goldmark/text"
	markdownutil "github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
	"opencsg.com/csghub-server/common/types"
)

var commentAudioExtensions = map[string]struct{}{
	".mp3": {}, ".wav": {}, ".ogg": {}, ".flac": {}, ".aac": {}, ".m4a": {},
	".wma": {}, ".opus": {}, ".weba": {},
}

var commentVideoExtensions = map[string]struct{}{
	".mp4": {}, ".avi": {}, ".mov": {}, ".wmv": {}, ".flv": {}, ".mkv": {},
	".webm": {}, ".m4v": {}, ".m3u8": {}, ".3gp": {}, ".mpg": {}, ".mpeg": {},
	".rm": {}, ".rmvb": {}, ".ts": {}, ".vob": {},
}

type commentMediaIdentity struct {
	mediaType types.MediaType
	url       string
}

// ExtractCommentMediaRefs parses comment content and returns deduplicated media
// refs. Every media URL must point at the configured public bucket and use a
// UUID object name, which identifies immutable uploaded content. It fails
// closed on external or mutable URLs, unsupported types, or when
// len(refs) > maxRefs.
func ExtractCommentMediaRefs(content, publicBucket, publicEndpoint string, maxRefs int) ([]types.MediaRef, error) {
	return ExtractEnabledCommentMediaRefs(content, publicBucket, publicEndpoint, maxRefs, true, true)
}

// ExtractEnabledCommentMediaRefs parses and validates only media types whose
// moderation feature is enabled. Disabled types are ignored so turning a
// checker off preserves the pre-moderation comment contract.
func ExtractEnabledCommentMediaRefs(
	content, publicBucket, publicEndpoint string,
	maxRefs int,
	imagesEnabled, avEnabled bool,
) ([]types.MediaRef, error) {
	source := []byte(content)
	document := goldmark.DefaultParser().Parse(markdowntext.NewReader(source))
	refs := make([]types.MediaRef, 0)
	seen := make(map[commentMediaIdentity]struct{})
	if err := extractMarkdownImageRefs(document, publicBucket, publicEndpoint, imagesEnabled, avEnabled, &refs, seen); err != nil {
		return nil, err
	}
	if err := extractHTMLMediaRefs(source, document, publicBucket, publicEndpoint, imagesEnabled, avEnabled, &refs, seen); err != nil {
		return nil, err
	}
	if maxRefs > 0 && len(refs) > maxRefs {
		return nil, fmt.Errorf("comment references too many media items: %d > %d", len(refs), maxRefs)
	}
	if len(refs) == 0 {
		return nil, nil
	}
	return refs, nil
}

func extractMarkdownImageRefs(
	document markdownast.Node,
	publicBucket, publicEndpoint string,
	imagesEnabled, avEnabled bool,
	refs *[]types.MediaRef,
	seen map[commentMediaIdentity]struct{},
) error {
	return markdownast.Walk(document, func(node markdownast.Node, entering bool) (markdownast.WalkStatus, error) {
		if !entering || node.Kind() != markdownast.KindImage {
			return markdownast.WalkContinue, nil
		}
		image := node.(*markdownast.Image)
		if len(image.Destination) != 0 {
			destination := markdownutil.UnescapePunctuations(image.Destination)
			rawURL := string(destination)
			// A Markdown image node may actually reference an audio or video
			// asset (e.g. ![](https://.../x.mp4)), which many frontends render
			// as <video>/<audio>. Classify it by extension so it is routed to
			// asynchronous media moderation instead of synchronous image check
			// (which would reject .mp4/.mp3 as "format not support").
			mediaType := mediaTypeFromExtension(rawURL)
			if mediaType == "" {
				mediaType = types.MediaTypeImage
			}
			if !commentMediaTypeEnabled(mediaType, imagesEnabled, avEnabled) {
				return markdownast.WalkContinue, nil
			}
			if mediaType == types.MediaTypeAudio || mediaType == types.MediaTypeVideo {
				// Audio/video must resolve to the configured public bucket so
				// ResourceKey() can be computed; fail closed otherwise.
				ref, err := commentMediaRef(rawURL, publicBucket, publicEndpoint, mediaType)
				if err != nil {
					return markdownast.WalkStop, err
				}
				appendUniqueMediaRef(refs, seen, ref)
				return markdownast.WalkContinue, nil
			}
			ref, err := commentMediaRef(rawURL, publicBucket, publicEndpoint, mediaType)
			if err != nil {
				return markdownast.WalkStop, err
			}
			appendUniqueMediaRef(refs, seen, ref)
		}
		return markdownast.WalkContinue, nil
	})
}

func extractHTMLMediaRefs(
	source []byte,
	document markdownast.Node,
	publicBucket, publicEndpoint string,
	imagesEnabled, avEnabled bool,
	refs *[]types.MediaRef,
	seen map[commentMediaIdentity]struct{},
) error {
	var rawHTML strings.Builder
	if err := markdownast.Walk(document, func(node markdownast.Node, entering bool) (markdownast.WalkStatus, error) {
		if !entering {
			return markdownast.WalkContinue, nil
		}
		switch node := node.(type) {
		case *markdownast.RawHTML:
			for index := 0; index < node.Segments.Len(); index++ {
				segment := node.Segments.At(index)
				rawHTML.Write(segment.Value(source))
			}
		case *markdownast.HTMLBlock:
			for index := 0; index < node.Lines().Len(); index++ {
				segment := node.Lines().At(index)
				rawHTML.Write(segment.Value(source))
			}
			if node.HasClosure() {
				rawHTML.Write(node.ClosureLine.Value(source))
			}
		}
		return markdownast.WalkContinue, nil
	}); err != nil {
		return err
	}

	tokenizer := html.NewTokenizer(strings.NewReader(rawHTML.String()))
	parents := make([]string, 0)
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return nil
			}
			return fmt.Errorf("parse comment HTML: %w", tokenizer.Err())
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			tag := strings.ToLower(token.Data)
			if tag == "audio" || tag == "video" || tag == "source" || tag == "img" {
				if err := appendHTMLMediaRef(tag, token.Attr, parents, publicBucket, publicEndpoint, imagesEnabled, avEnabled, refs, seen); err != nil {
					return err
				}
			}
			if tokenType == html.StartTagToken {
				parents = append(parents, tag)
			}
		case html.EndTagToken:
			tag := strings.ToLower(tokenizer.Token().Data)
			for index := len(parents) - 1; index >= 0; index-- {
				if parents[index] == tag {
					parents = parents[:index]
					break
				}
			}
		}
	}
}

func appendHTMLMediaRef(
	tag string,
	attrs []html.Attribute,
	parents []string,
	publicBucket, publicEndpoint string,
	imagesEnabled, avEnabled bool,
	refs *[]types.MediaRef,
	seen map[commentMediaIdentity]struct{},
) error {
	rawURL := firstHTMLAttribute(attrs, "src")
	if rawURL == "" {
		return nil
	}
	if tag == "img" {
		// An <img> may point at an audio/video asset by extension; classify it
		// so it is routed to media moderation instead of the image check.
		mediaType := mediaTypeFromExtension(rawURL)
		if mediaType == "" {
			mediaType = types.MediaTypeImage
		}
		if !commentMediaTypeEnabled(mediaType, imagesEnabled, avEnabled) {
			return nil
		}
		if mediaType == types.MediaTypeAudio || mediaType == types.MediaTypeVideo {
			ref, err := commentMediaRef(rawURL, publicBucket, publicEndpoint, mediaType)
			if err != nil {
				return err
			}
			appendUniqueMediaRef(refs, seen, ref)
			return nil
		}
		ref, err := commentMediaRef(rawURL, publicBucket, publicEndpoint, mediaType)
		if err != nil {
			return err
		}
		appendUniqueMediaRef(refs, seen, ref)
		return nil
	}
	mediaType, err := htmlMediaType(tag, rawURL, firstHTMLAttribute(attrs, "type"), parents)
	if err != nil {
		if !avEnabled {
			return nil
		}
		return err
	}
	if !commentMediaTypeEnabled(mediaType, imagesEnabled, avEnabled) {
		return nil
	}
	ref, err := commentMediaRef(rawURL, publicBucket, publicEndpoint, mediaType)
	if err != nil {
		return err
	}
	appendUniqueMediaRef(refs, seen, ref)
	return nil
}

func commentMediaTypeEnabled(mediaType types.MediaType, imagesEnabled, avEnabled bool) bool {
	if mediaType == types.MediaTypeImage {
		return imagesEnabled
	}
	return avEnabled
}

func commentMediaRef(rawURL, publicBucket, publicEndpoint string, mediaType types.MediaType) (types.MediaRef, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return types.MediaRef{}, fmt.Errorf("invalid media URL %q", rawURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return types.MediaRef{}, fmt.Errorf("media URL %q must use http or https", rawURL)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return types.MediaRef{}, fmt.Errorf("media URL %q must not contain query parameters or fragments", rawURL)
	}
	bucket, objectKey := commentMediaBucketAndObject(parsed, publicBucket, publicEndpoint)
	if bucket == "" || objectKey == "" {
		return types.MediaRef{}, fmt.Errorf(
			"media URL must use public bucket %q at public endpoint %q",
			publicBucket,
			publicEndpoint,
		)
	}
	resourceID := strings.TrimSuffix(path.Base(objectKey), path.Ext(objectKey))
	if _, err := uuid.Parse(resourceID); err != nil {
		return types.MediaRef{}, fmt.Errorf("media URL %q must use an immutable UUID object key", rawURL)
	}
	return types.MediaRef{
		URL: rawURL, Bucket: bucket, ObjectKey: objectKey, Type: mediaType,
	}, nil
}

func commentMediaBucketAndObject(parsed *url.URL, publicBucket, publicEndpoint string) (string, string) {
	endpointAuthority := commentMediaEndpointAuthority(publicEndpoint)
	if publicBucket == "" || endpointAuthority == "" || parsed.User != nil {
		return "", ""
	}
	requestAuthority := strings.ToLower(parsed.Host)
	virtualAuthority := strings.ToLower(publicBucket + "." + endpointAuthority)
	objectKey := strings.TrimPrefix(parsed.Path, "/")
	if requestAuthority == virtualAuthority {
		return publicBucket, objectKey
	}
	parts := strings.SplitN(objectKey, "/", 2)
	if requestAuthority == endpointAuthority && len(parts) == 2 && strings.EqualFold(parts[0], publicBucket) {
		return publicBucket, parts[1]
	}
	return "", ""
}

func commentMediaEndpointAuthority(publicEndpoint string) string {
	endpoint := strings.TrimSpace(publicEndpoint)
	if endpoint == "" {
		return ""
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "//" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") {
		return ""
	}
	return strings.ToLower(parsed.Host)
}

func htmlMediaType(tag, rawURL, mimeType string, parents []string) (types.MediaType, error) {
	extensionType := mediaTypeFromExtension(rawURL)
	explicitMIME := strings.TrimSpace(mimeType)
	mimeMediaType := mediaTypeFromMIME(explicitMIME)
	if explicitMIME != "" && mimeMediaType == "" {
		return "", fmt.Errorf("unknown source media type %q", mimeType)
	}
	switch tag {
	case "audio":
		if extensionType != types.MediaTypeAudio || (mimeMediaType != "" && mimeMediaType != types.MediaTypeAudio) {
			return "", fmt.Errorf("unsupported audio media URL %q", rawURL)
		}
		return types.MediaTypeAudio, nil
	case "video":
		if extensionType != types.MediaTypeVideo || (mimeMediaType != "" && mimeMediaType != types.MediaTypeVideo) {
			return "", fmt.Errorf("unsupported video media URL %q", rawURL)
		}
		return types.MediaTypeVideo, nil
	}

	parentType := parentHTMLMediaType(parents)
	resolved := types.MediaType("")
	for _, candidate := range []types.MediaType{mimeMediaType, parentType, extensionType} {
		if candidate == "" {
			continue
		}
		if resolved != "" && resolved != candidate {
			return "", fmt.Errorf("conflicting source media type for URL %q", rawURL)
		}
		resolved = candidate
	}
	if resolved != "" {
		return resolved, nil
	}
	return "", fmt.Errorf("unknown source media type for URL %q", rawURL)
}

func parentHTMLMediaType(parents []string) types.MediaType {
	for index := len(parents) - 1; index >= 0; index-- {
		switch parents[index] {
		case "audio":
			return types.MediaTypeAudio
		case "video":
			return types.MediaTypeVideo
		}
	}
	return ""
}

func mediaTypeFromExtension(rawURL string) types.MediaType {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if _, ok := commentAudioExtensions[ext]; ok {
		return types.MediaTypeAudio
	}
	if _, ok := commentVideoExtensions[ext]; ok {
		return types.MediaTypeVideo
	}
	return ""
}

func mediaTypeFromMIME(mimeType string) types.MediaType {
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	if strings.HasPrefix(mediaType, "audio/") {
		return types.MediaTypeAudio
	}
	if strings.HasPrefix(mediaType, "video/") {
		return types.MediaTypeVideo
	}
	return ""
}

func firstHTMLAttribute(attrs []html.Attribute, name string) string {
	for _, attr := range attrs {
		if attr.Namespace == "" && strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}

func appendUniqueMediaRef(
	refs *[]types.MediaRef,
	seen map[commentMediaIdentity]struct{},
	ref types.MediaRef,
) {
	identity := commentMediaIdentity{mediaType: ref.Type, url: ref.URL}
	if _, ok := seen[identity]; ok {
		return
	}
	seen[identity] = struct{}{}
	*refs = append(*refs, ref)
}
