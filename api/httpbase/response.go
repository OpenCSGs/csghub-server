package httpbase

import (
	"net/http"

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
	err, ok := errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusBadRequest, R{
			Code:    customErr.Code(),
			Msg:     customErr.Error(),
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
	err, ok := errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusInternalServerError, R{
			Code:    customErr.Code(),
			Msg:     customErr.Error(),
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
	err, ok := errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusUnauthorized, R{
			Code:    customErr.Code(),
			Msg:     customErr.Error(),
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
	err, ok := errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusForbidden, R{
			Code:    customErr.Code(),
			Msg:     customErr.Error(),
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
	err, ok := errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		c.PureJSON(http.StatusNotFound, R{
			Code:    customErr.Code(),
			Msg:     customErr.Error(),
			Context: customErr.Context(),
		})
		return
	}
	c.PureJSON(http.StatusNotFound, R{
		Msg: err.Error(),
	})
}

// ConflictError responds with a JSON-formatted error message and HTTP 409 status code.
// Used when the request conflicts with the current state of the server.
//
// Example:
//
//	ConflictError(c, errorx.ErrCannotRemoveLastAdmin)
func ConflictError(c *gin.Context, err error) {
	resp := R{}
	err, ok := errorx.GetFirstCustomError(err)
	if ok {
		customErr := err.(errorx.CustomError)
		resp.Code = customErr.Code()
		resp.Msg = customErr.Error()
		resp.Context = customErr.Context()
	} else {
		resp.Msg = err.Error()
	}
	c.PureJSON(http.StatusConflict, resp)
}

// R is the response envelope
type R struct {
	Code  string `json:"code,omitempty"`
	Msg   string `json:"msg"`
	Data  any    `json:"data,omitempty"`
	Total int    `json:"total,omitempty"` // Total number of items, used in paginated responses
	// error context msg
	Context map[string]interface{} `json:"context,omitempty"`
}

type I18nOptionsMap map[i18n.I18nOptionsKey]string
