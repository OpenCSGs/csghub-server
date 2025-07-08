package httpbase

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/i18n"
)

// OK responds the client with standard JSON.
//
// Example:
// * ok(c, something)
// * ok(c, nil)
func OK(c *gin.Context, data interface{}) {
	respData := R{
		Msg:  "OK",
		Data: data,
	}
	if c.Request == nil || c.Request.Header == nil {
		c.PureJSON(http.StatusOK, respData)
		return
	}
	lang := c.GetHeader("Accept-Language")
	modifiedData := i18n.TranslateInterface(data, lang)
	respData.Data = modifiedData
	c.PureJSON(http.StatusOK, respData)
}

// OK responds the client with standard JSON.
//
// Example:
// * ok(c, something)
// * ok(c, nil)
func OKWithTotal(c *gin.Context, data interface{}, total int) {
	respData := R{
		Msg:   "OK",
		Data:  data,
		Total: total,
	}
	if c.Request == nil || c.Request.Header == nil {
		c.PureJSON(http.StatusOK, respData)
		return
	}
	lang := c.GetHeader("Accept-Language")
	modifiedData := i18n.TranslateInterface(data, lang)
	respData.Data = modifiedData
	c.PureJSON(http.StatusOK, respData)
}

// BadRequest responds with a JSON-formatted error message.
//
// Example:
//
//	BadRequest(c, "Invalid request parameters")
func BadRequest(c *gin.Context, errMsg string) {
	// get caller message
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		structName := parseFuncName(funcName)
		c.Set("Error-Handler-Name", structName)
	}
	c.PureJSON(http.StatusBadRequest, R{
		Msg: errMsg,
	})
}

// BadRequest responds with a JSON-formatted error message.
//
// Example:
//
//	BadRequest(c, "Invalid request parameters")
func BadRequestWithExt(c *gin.Context, err error) {
	// get caller message
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		structName := parseFuncName(funcName)
		c.Set("Error-Handler-Name", structName)
	}
	err, ok = errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusBadRequest, R{
			Msg:     customErr.Code(),
			Context: customErr.Context(),
		})
		return
	}
	c.PureJSON(http.StatusBadRequest, R{
		Msg: err.Error(),
	})
}

// ServerError responds with a JSON-formatted error message.
//
// Example:
//
//	ServerError(c, errors.New("internal server error"))
func ServerError(c *gin.Context, err error) {
	// get caller message
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		structName := parseFuncName(funcName)
		c.Set("Error-Handler-Name", structName)
	}
	err, ok = errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusInternalServerError, R{
			Msg:     customErr.Code(),
			Context: customErr.Context(),
		})
		return
	}
	c.PureJSON(http.StatusInternalServerError, R{
		Msg: err.Error(),
	})
}

// UnauthorizedError if the client is not authenticated or the authentication is invalid.
// Like user not login, for example.
//
// Response Example:
//
//	UnauthorizedError(c, errors.New("permission denied"))
func UnauthorizedError(c *gin.Context, err error) {
	// get caller message
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		structName := parseFuncName(funcName)
		c.Set("Error-Handler-Name", structName)
	}
	err, ok = errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusUnauthorized, R{
			Msg:     customErr.Code(),
			Context: customErr.Context(),
		})
		return
	}
	c.PureJSON(http.StatusUnauthorized, R{
		Msg: err.Error(),
	})
}

// ForbiddenError if the client is authenticated but does not have enough permissions.
func ForbiddenError(c *gin.Context, err error) {
	// get caller message
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		structName := parseFuncName(funcName)
		c.Set("Error-Handler-Name", structName)
	}
	err, ok = errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusForbidden, R{
			Msg:     customErr.Code(),
			Context: customErr.Context(),
		})
		return
	}
	c.PureJSON(http.StatusForbidden, R{
		Msg: err.Error(),
	})
}

// NotFoundError responds with a JSON-formatted error message.
//
// Example:
//
//	NotFoundError(c, errors.New("permission denied"))
func NotFoundError(c *gin.Context, err error) {
	// get caller message
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		structName := parseFuncName(funcName)
		c.Set("Error-Handler-Name", structName)
	}
	err, ok = errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusNotFound, R{
			Msg:     customErr.Code(),
			Context: customErr.Context(),
		})
		return
	}
	c.PureJSON(http.StatusNotFound, R{
		Msg: err.Error(),
	})
}

// R is the response envelope
type R struct {
	Code  int    `json:"code,omitempty"`
	Msg   string `json:"msg"`
	Data  any    `json:"data,omitempty"`
	Total int    `json:"total,omitempty"` // Total number of items, used in paginated responses
	// error context msg
	Context map[string]interface{} `json:"context,omitempty"`
}

type I18nOptionsMap map[i18n.I18nOptionsKey]string

func parseFuncName(funcName string) string {
	// 1. get strcut name and method name
	structName := extractStructAndMethod(funcName)
	// 2. remove "handler" suffix (if exist)
	if strings.HasSuffix(structName, "handler") {
		structName = strings.TrimSuffix(structName, "handler")
	} else {
		// maybe middleware.*, set it to empty
		structName = ""
	}
	return structName
}

// extractStructAndMethod extracts the struct name from a function name.
func extractStructAndMethod(funcName string) string {
	parts := strings.Split(funcName, ".")
	if len(parts) < 2 {
		return ""
	}
	structName := parts[len(parts)-2]
	// Remove the package name if it exists
	if strings.HasPrefix(structName, "(*") && strings.HasSuffix(structName, ")") {
		structName = structName[2 : len(structName)-1]
	}
	structName = strings.ToLower(structName)
	return structName
}
