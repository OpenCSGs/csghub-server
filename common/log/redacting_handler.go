package log

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const redactedValue = "[REDACTED]"

var secretTextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)((?:bearer|basic)\s+)[^\s,;"']+`),
	regexp.MustCompile(`(?i)((?:authorization|access[_-]?token|refresh[_-]?token|id[_-]?token|private[_-]?token|personal[_-]?access[_-]?token|api[_-]?key|jwt[_-]?token|jwt|token|password|passwd|client[_-]?secret|cookie)["']?\s*[=:]\s*)(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s&,;}]+)`),
}

var credentialPathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(/(?:api/v1/)?jwt/).+`),
	regexp.MustCompile(`^(/(?:api/v1/)?token/)[^/]+(/quotas)?/?$`),
}
var urlTextPattern = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s"'<>]+`)

// CredentialFingerprint returns a stable, irreversible correlation value for a credential.
func CredentialFingerprint(kind, credential string) string {
	if credential == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(credential))
	return kind + ":sha256:" + hex.EncodeToString(sum[:8])
}

// RedactingHandler removes credentials from structured log attributes before
// forwarding records to the wrapped handler.
type RedactingHandler struct {
	slog.Handler
}

// NewRedactingHandler wraps handler with credential redaction.
func NewRedactingHandler(handler slog.Handler) slog.Handler {
	return &RedactingHandler{Handler: handler}
}

func (h *RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, SafeText(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactAttr(attr))
		return true
	})
	return h.Handler.Handle(ctx, redacted)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(attr))
	}
	return &RedactingHandler{Handler: h.Handler.WithAttrs(redacted)}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{Handler: h.Handler.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if isSensitiveLogKey(attr.Key) {
		return slog.String(attr.Key, redactedValue)
	}

	switch attr.Value.Kind() {
	case slog.KindGroup:
		group := attr.Value.Group()
		redacted := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			redacted = append(redacted, redactAttr(child))
		}
		return slog.Group(attr.Key, attrsToAny(redacted)...)
	case slog.KindString:
		if isURLLogKey(attr.Key) {
			return slog.String(attr.Key, SafeURLString(attr.Value.String()))
		}
		return slog.String(attr.Key, SafeText(attr.Value.String()))
	case slog.KindAny:
		return slog.Any(attr.Key, redactAny(attr.Value.Any(), 0))
	}
	return attr
}

func attrsToAny(attrs []slog.Attr) []any {
	values := make([]any, len(attrs))
	for i := range attrs {
		values[i] = attrs[i]
	}
	return values
}

func redactAny(value any, depth int) any {
	if value == nil || depth > 8 {
		return value
	}
	if requestURL, ok := value.(*url.URL); ok {
		return SafeURL(requestURL)
	}
	if request, ok := value.(*http.Request); ok {
		return map[string]any{
			"method": request.Method,
			"url":    SafeURL(request.URL),
			"header": SafeHeaders(request.Header),
		}
	}
	if headers, ok := value.(http.Header); ok {
		return SafeHeaders(headers)
	}
	switch typed := value.(type) {
	case error:
		return SafeError(typed)
	case time.Time, time.Duration:
		return typed
	}
	redacted, _ := redactReflect(reflect.ValueOf(value), depth)
	return redacted
}

func redactReflect(value reflect.Value, depth int) (any, bool) {
	if !value.IsValid() {
		return nil, false
	}
	if depth > 16 {
		return redactedValue, true
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		result := make(map[string]any, value.NumField())
		changed := false
		typeOfValue := value.Type()
		for i := 0; i < value.NumField(); i++ {
			fieldType := typeOfValue.Field(i)
			if !fieldType.IsExported() {
				continue
			}
			name := fieldType.Name
			tag := strings.TrimSpace(strings.Split(fieldType.Tag.Get("json"), ",")[0])
			if tag == "-" {
				continue
			}
			if tag != "" {
				name = tag
			}
			if isSensitiveLogKey(name) {
				result[name] = redactedValue
				changed = true
				continue
			}
			fieldValue, fieldChanged := redactReflect(value.Field(i), depth+1)
			result[name] = fieldValue
			changed = changed || fieldChanged
		}
		if !changed && value.CanInterface() {
			return value.Interface(), false
		}
		return result, changed
	case reflect.Map:
		result := make(map[string]any, value.Len())
		changed := false
		iterator := value.MapRange()
		for iterator.Next() {
			key := fmt.Sprint(iterator.Key().Interface())
			if isSensitiveLogKey(key) {
				result[key] = redactedValue
				changed = true
				continue
			}
			mapValue, mapChanged := redactReflect(iterator.Value(), depth+1)
			result[key] = mapValue
			changed = changed || mapChanged
		}
		if !changed && value.CanInterface() {
			return value.Interface(), false
		}
		return result, changed
	case reflect.Slice, reflect.Array:
		result := make([]any, value.Len())
		changed := false
		for i := 0; i < value.Len(); i++ {
			item, itemChanged := redactReflect(value.Index(i), depth+1)
			result[i] = item
			changed = changed || itemChanged
		}
		if !changed && value.CanInterface() {
			return value.Interface(), false
		}
		return result, changed
	case reflect.String:
		original := value.String()
		redacted := SafeText(original)
		return redacted, redacted != original
	default:
		if value.CanInterface() {
			return value.Interface(), false
		}
		return fmt.Sprint(value), false
	}
}

// SafeHeaders returns a copy of headers with credential-bearing values removed.
func SafeHeaders(headers http.Header) http.Header {
	redacted := headers.Clone()
	for key := range redacted {
		if isSensitiveHeader(key) {
			redacted[key] = []string{redactedValue}
		}
	}
	return redacted
}

// SafeURL returns a copy with credential-bearing paths, queries and userinfo removed.
func SafeURL(requestURL *url.URL) *url.URL {
	if requestURL == nil {
		return nil
	}
	redacted := *requestURL
	if redacted.User != nil {
		redacted.User = url.User(redactedValue)
	}
	redacted.Path = safeCredentialPath(redacted.Path)
	if redacted.Path == requestURL.Path {
		if escaped := safeCredentialPath(requestURL.EscapedPath()); escaped != requestURL.EscapedPath() {
			redacted.Path, _ = url.PathUnescape(escaped)
		}
	}
	if redacted.Path != requestURL.Path {
		redacted.RawPath = ""
	}
	callbackPath := strings.TrimRight(redacted.Path, "/")
	isOAuthCallback := strings.HasSuffix(callbackPath, "/oauth/callback") || strings.HasSuffix(callbackPath, "/callback/casdoor")
	query := redacted.Query()
	for key := range query {
		oauthCallbackValue := isOAuthCallback &&
			(normalizeLogKey(key) == "code" || normalizeLogKey(key) == "state")
		if isSensitiveLogKey(key) || oauthCallbackValue {
			query[key] = []string{redactedValue}
		}
	}
	redacted.RawQuery = query.Encode()
	return &redacted
}

func safeCredentialPath(path string) string {
	for _, pattern := range credentialPathPatterns {
		path = pattern.ReplaceAllString(path, `${1}`+redactedValue+`${2}`)
	}
	return path
}

// SafeURLString removes URL credentials and suppresses malformed URLs.
func SafeURLString(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return redactedValue
	}
	return SafeURL(parsed).String()
}

// SafeText removes common credential representations from log messages and errors.
func SafeText(value string) string {
	redacted := value
	if strings.Contains(value, "://") {
		redacted = urlTextPattern.ReplaceAllStringFunc(value, SafeURLString)
	}
	for _, pattern := range secretTextPatterns {
		redacted = pattern.ReplaceAllString(redacted, `${1}`+redactedValue)
	}
	return redacted
}

// SafeError removes credentials from URL errors before they are logged or wrapped.
func SafeError(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && err == urlError {
		return &url.Error{
			Op:  urlError.Op,
			URL: SafeURLString(urlError.URL),
			Err: SafeError(urlError.Err),
		}
	}
	message := SafeText(err.Error())
	if message == err.Error() {
		return err
	}
	return &safeError{message: message, cause: err}
}

type safeError struct {
	message string
	cause   error
}

func (e *safeError) Error() string { return e.message }

func (e *safeError) Unwrap() error { return e.cause }

func isURLLogKey(key string) bool {
	normalized := normalizeLogKey(key)
	return strings.HasSuffix(normalized, "url") || strings.HasSuffix(normalized, "uri") ||
		normalized == "path" || normalized == "request" || normalized == "endpoint"
}

func isSensitiveHeader(key string) bool {
	normalized := normalizeLogKey(key)
	if strings.HasSuffix(normalized, "token") || strings.HasSuffix(normalized, "apikey") ||
		strings.HasSuffix(normalized, "secret") || strings.HasSuffix(normalized, "password") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "api-key", "x-goog-api-key", "x-auth-token", "x-access-token", "private-token", "gitlab-shell-api-request", "x-amz-credential", "x-amz-security-token":
		return true
	default:
		return false
	}
}

func isSensitiveLogKey(key string) bool {
	normalized := normalizeLogKey(key)
	if strings.HasSuffix(normalized, "token") || strings.HasSuffix(normalized, "apikey") || strings.HasSuffix(normalized, "password") ||
		strings.HasSuffix(normalized, "secret") || strings.HasSuffix(normalized, "credential") {
		return true
	}
	switch normalized {
	case "token", "tokenget", "accesstoken", "refreshtoken", "idtoken", "privatetoken", "personalaccesstoken", "jwt", "apikey", "authorization", "auth", "authheader", "cookie", "setcookie", "password", "passwd", "secret", "credential", "credentials", "clientsecret", "signature", "sig":
		return true
	default:
		return false
	}
}

func normalizeLogKey(key string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, key)
}
