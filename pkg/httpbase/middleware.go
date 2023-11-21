package httpbase

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/log"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/nanmu42/limitio"
	"github.com/signalsciences/ac/acascii"
	"github.com/valyala/bytebufferpool"
	"go.uber.org/zap"
)

const (
	ctxSkipLoggingKey   = "skipLogging"
	ctxRequestAuditKey  = "requestAudit"
	ctxResponseAuditKey = "responseAudit"
)

// SkipLogging marks when we don't want logging
func SkipLogging(c *gin.Context) {
	c.Set(ctxSkipLoggingKey, true)
}

// Middleware is where common, business-non-related http middlewares and handlers lives.
// It acts as easy building blocks for HTTP services.
type Middleware struct {
	Logger log.Logger
	// see Middleware PayloadAuditLog
	AuditResponse bool
	// used for logging and Sentry report.
	// An empty UserIdentity must be returned if it's impossible to identify the user.
	UserIdentifier func(c *gin.Context) UserIdentity
}

// UserIdentity contains fields that can identify a user.
// All fields are optional but at least one field must be specified.
type UserIdentity struct {
	// internal ID of the user
	ID int
}

func (i UserIdentity) IsEmpty() bool {
	return i == UserIdentity{}
}

// HelloHandler says hello world,
// also stands for health check.
// @Summary says hello world
// @Success 200 {string} string "hello world"
// @Router /api/hello [get]
func (m *Middleware) HelloHandler(c *gin.Context) {
	SkipLogging(c)
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusNoContent)
		return
	}
	OK(c, "hello world!")
}

// NotFoundHandler 404 not found handler
func (m *Middleware) NotFoundHandler(c *gin.Context) {
	c.PureJSON(http.StatusNotFound, R{
		Code: http.StatusNotFound,
		Msg:  http.StatusText(http.StatusNotFound),
		Data: nil,
	})
}

// MethodNotAllowedHandler 405 method not allowed handler
func (m *Middleware) MethodNotAllowedHandler(c *gin.Context) {
	c.PureJSON(http.StatusMethodNotAllowed, R{
		Code: http.StatusMethodNotAllowed,
		Msg:  http.StatusText(http.StatusMethodNotAllowed),
		Data: nil,
	})
}

func (m *Middleware) RobotsTXTHandler(c *gin.Context) {
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusNoContent)
		return
	}
	c.String(http.StatusOK, `User-agent: *
Disallow: /`)
}

// Recovery recover from panic and log
func (m *Middleware) Recovery(c *gin.Context) {
	defer func() {
		err := recover()
		if err == nil {
			// relax
			return
		}

		var (
			user     UserIdentity
			stack    = debug.Stack()
			clientIP = c.ClientIP()
			logger   = m.Logger
		)
		if m.UserIdentifier != nil {
			user = m.UserIdentifier(c)
		}
		if !user.IsEmpty() {
			logger = logger.With(zap.Int("userId", user.ID))
		}

		logger.Error("panic recovered!",
			log.Any("panic", err),
			log.ByteString("stack", stack),
			log.String("method", c.Request.Method),
			log.String("host", c.Request.Host),
			log.String("path", c.Request.URL.Path),
			log.String("clientIP", clientIP),
			log.String("referer", c.Request.Referer()),
			log.String("UA", c.Request.UserAgent()),
			log.Strings("errors", c.Errors.Errors()),
		)

		hub := sentry.CurrentHub().Clone()
		hub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetContext("request", map[string]interface{}{
				"method":   c.Request.Method,
				"host":     c.Request.Host,
				"path":     c.Request.URL.Path,
				"clientIP": clientIP,
				"UA":       c.Request.UserAgent(),
				"referer":  c.Request.Referer(),
				"errors":   c.Errors.Errors(),
			})
			scope.SetTag("endpoint", fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path))

			if !user.IsEmpty() {
				scope.SetUser(sentry.User{
					ID:        strconv.Itoa(user.ID),
					IPAddress: clientIP,
				})
			}
		})
		hub.Recover(err)

		if !c.Writer.Written() {
			c.PureJSON(http.StatusInternalServerError, R{
				Code: http.StatusInternalServerError,
				Msg:  http.StatusText(http.StatusInternalServerError),
				Data: nil,
			})
		}
	}()

	c.Next()
}

// errResponse wraps the R struct
// it adds msgKey and
type errResponse struct {
	R
	MsgKey string `json:"msgKey"`
}

// Error attaches errors submitted by c.Error() into request log via RequestLog middleware.
// When there's no HTTP body written, Error responds with the last error submitted.
// You may use this feature to "conceal" the real, detailed error.
func (m *Middleware) Error(c *gin.Context) {
	c.Next()

	err := c.Errors.Last()
	if err == nil {
		return
	}
	// abort if there's already a response body
	if c.Writer.Written() {
		return
	}

	var (
		statusCode int
		resp       errResponse
		apiErr     *Error
	)

	switch {
	case err.IsType(gin.ErrorTypeBind):
		resp.Code = CodeBadBinding
		resp.MsgKey = msgKeyBindingError
		statusCode = http.StatusNotAcceptable
	case errors.As(err.Err, &apiErr):
		resp.Code = apiErr.Code()
		resp.MsgKey = apiErr.MsgKey()
		statusCode = apiErr.StatusCode()
	case errors.As(err.Err, &validation.Errors{}):
		resp.Code = CodeDynamicalFormInputError
		resp.MsgKey = msgKeyDynamicalFormInputError
		validationErrors := utils.UnwrapError(err.Err)
		resp.Msg = validationErrors.Error()
		resp.Data = validationErrors.(validation.Errors)
		statusCode = 200
	default:
		resp.Code = CodeGeneralError
		resp.MsgKey = msgKeyGeneralError
		statusCode = http.StatusOK
	}

	resp.Msg = err.Error()
	c.PureJSON(statusCode, resp)
}

// RequestLog logs the status of every request
func (m *Middleware) RequestLog(c *gin.Context) {
	startedAt := time.Now()

	c.Next()

	// sometimes we just don't want log
	if c.GetBool(ctxSkipLoggingKey) {
		return
	}

	var (
		user     UserIdentity
		latency  = time.Since(startedAt)
		logger   = m.Logger
		clientIP = c.ClientIP()
	)

	if m.UserIdentifier != nil {
		user = m.UserIdentifier(c)
	}
	if !user.IsEmpty() {
		logger = logger.With(zap.Int("userId", user.ID))
	}

	if reqBody, ok := c.Get(ctxRequestAuditKey); ok {
		logger = logger.With(log.Stringp("requestBody", reqBody.(*string)))
	}
	if respBody, ok := c.Get(ctxResponseAuditKey); ok {
		logger = logger.With(log.Stringp("responseBody", respBody.(*string)))
	}

	logger.For(c.Request.Context()).
		Info("APIAuditLog",
			log.String("method", c.Request.Method),
			log.String("host", c.Request.Host),
			log.String("referer", c.Request.Referer()),
			log.String("path", c.Request.URL.Path),
			log.String("clientIP", clientIP),
			log.String("UA", c.Request.UserAgent()),
			log.Int("status", c.Writer.Status()),
			log.Duration("latency", latency),
			log.Int64("reqLength", c.Request.ContentLength),
			log.Int("resLength", c.Writer.Size()),
			log.Strings("errors", c.Errors.Errors()),
		)

	// report error to sentry if applicable
	if len(c.Errors) == 0 {
		// no error to report, relax
		return
	}
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetContext("request", map[string]interface{}{
			"method":    c.Request.Method,
			"host":      c.Request.Host,
			"path":      c.Request.URL.Path,
			"clientIP":  clientIP,
			"UA":        c.Request.UserAgent(),
			"referer":   c.Request.Referer(),
			"errors":    c.Errors.Errors(),
			"status":    c.Writer.Status(),
			"latencyMs": latency.Milliseconds(),
			"reqLength": c.Request.ContentLength,
			"resLength": c.Writer.Size(),
		})
		scope.SetTag("endpoint", fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path))

		if !user.IsEmpty() {
			scope.SetUser(sentry.User{
				ID:        strconv.Itoa(user.ID),
				IPAddress: clientIP,
			})
		}
	})

	// We don't use sentry.CaptureException() since the related stack trace is plain useless.
	event := &sentry.Event{
		Level:   sentry.LevelInfo,
		Message: c.Errors[0].Error(),
	}
	hub.CaptureEvent(event)
}

// CORSMiddleware allows CORS request
func CORSMiddleware(c *gin.Context) {
	header := c.Writer.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Max-Age", "43200")
	header.Set("Access-Control-Allow-Methods", "POST")
	header.Set("Access-Control-Allow-Headers", "Content-Type, Token")

	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}

	c.Next()
}

// LimitRequestBody limits the request size
func (m *Middleware) LimitRequestBody(maxRequestBodyBytes int) func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.Request.ContentLength > int64(maxRequestBodyBytes) {
			_ = c.Error(fmt.Errorf("oversized payload by content length, got %d bytes, limit %d bytes", c.Request.ContentLength, maxRequestBodyBytes))

			c.Abort()
			return
		}

		c.Request.Body = limitio.NewReadCloser(c.Request.Body, maxRequestBodyBytes, false)

		c.Next()
	}
}

// PayloadAuditLog audits text request and response then logs them.
// This middleware replies on RequestLog to output,
// use it inside RequestLog so that RequestLog can see this one's product.
//
// Note:
//
// If there's a gzip middleware, use it outside this one so this one can see response before compressing.
func (m *Middleware) PayloadAuditLog() func(c *gin.Context) {
	const (
		RequestBodyMaxLength  = 512
		ResponseBodyMaxLength = 512
	)

	textPayloadMIME := []string{
		"application/json", "text/xml", "application/xml", "text/html",
		"text/richtext", "text/plain", "text/css", "text/x-script",
		"text/x-component", "text/x-markdown", "application/javascript",
	}
	MIMEChecker := acascii.MustCompileString(textPayloadMIME)

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead ||
			c.Request.Method == http.MethodOptions {
			return
		}

		reqBuf := bytebufferpool.Get()
		defer bytebufferpool.Put(reqBuf)
		limitedReqBuf := limitio.NewWriter(reqBuf, RequestBodyMaxLength, true)

		c.Request.Body = &readCloser{
			Reader: io.TeeReader(c.Request.Body, limitedReqBuf),
			Closer: c.Request.Body,
		}

		var respBuf *bytebufferpool.ByteBuffer
		if m.AuditResponse {
			respBuf = bytebufferpool.Get()
			defer bytebufferpool.Put(respBuf)
			limitedRespBuf := limitio.NewWriter(respBuf, ResponseBodyMaxLength, true)

			c.Writer = &logWriter{
				ResponseWriter: c.Writer,
				SavedBody:      limitedRespBuf,
			}
		}

		c.Next()

		// sometimes we just don't want log
		if c.GetBool(ctxSkipLoggingKey) {
			return
		}

		var (
			reqBody  string
			respBody string
		)

		if reqBuf.Len() > 0 {
			reqContentType := c.Request.Header.Get("Content-Type")
			if reqContentType == "" {
				reqContentType = http.DetectContentType(reqBuf.Bytes())
			}
			if reqContentType != "" && MIMEChecker.MatchString(reqContentType) {
				reqBody = reqBuf.String()
			} else {
				reqBody = "unsupported content type: " + reqContentType
			}
			c.Set(ctxRequestAuditKey, &reqBody)
		}

		//goland:noinspection ALL
		if m.AuditResponse && respBuf.Len() > 0 {
			if respContentType := c.Writer.Header().Get("Content-Type"); respContentType != "" && MIMEChecker.MatchString(respContentType) {
				//goland:noinspection ALL
				respBody = respBuf.String()
			} else {
				respBody = "unsupported content type: " + respContentType
			}

			c.Set(ctxResponseAuditKey, &respBody)
		}
	}
}

type readCloser struct {
	io.Reader
	io.Closer
}

type logWriter struct {
	gin.ResponseWriter
	SavedBody io.Writer
}

func (w *logWriter) Write(b []byte) (int, error) {
	_, _ = w.SavedBody.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *logWriter) WriteString(s string) (int, error) {
	_, _ = w.SavedBody.Write([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

// OK responds the client with standard JSON.
//
// Example:
// * ok(c, something)
// * ok(c, nil)
func OK(c *gin.Context, data interface{}) {
	c.PureJSON(http.StatusOK, R{
		Code: 0,
		Msg:  "OK",
		Data: data,
	})
}

// R is the response envelope
type R struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}
