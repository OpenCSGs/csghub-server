package httpbase

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OK responds the client with standard JSON.
//
// Example:
// * ok(c, something)
// * ok(c, nil)
func OK(c *gin.Context, data interface{}) {
	c.PureJSON(http.StatusOK, R{
		Msg:  "OK",
		Data: data,
	})
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

// ServerError responds with a JSON-formatted error message.
//
// Example:
//
//	ServerError(c, errors.New("internal server error"))
func ServerError(c *gin.Context, err error) {
	c.PureJSON(http.StatusInternalServerError, R{
		Msg: err.Error(),
	})
}

// R is the response envelope
type R struct {
	Code int    `json:"code,omitempty"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}
