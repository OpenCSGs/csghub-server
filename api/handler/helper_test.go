package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/component"
)

type GinTester struct {
	ginHandler gin.HandlerFunc
	gctx       *gin.Context
	ctx        context.Context
	response   *httptest.ResponseRecorder
	OKText     string // text of httpbase.OK
	_executed  bool
}

func NewGinTester() *GinTester {
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{
		URL: &url.URL{},
	}

	return &GinTester{
		ginHandler: nil,
		gctx:       ctx,
		ctx:        ctx.Request.Context(),
		response:   response,
		OKText:     "OK",
	}
}

func (g *GinTester) Handler(handler gin.HandlerFunc) {
	g.ginHandler = handler
}

func (g *GinTester) Execute() {
	g.ginHandler(g.gctx)
	g._executed = true
}
func (g *GinTester) WithUser() *GinTester {
	g.gctx.Set(httpbase.CurrentUserCtxVar, "u")
	return g
}

func (g *GinTester) WithParam(key string, value string) *GinTester {
	params := g.gctx.Params
	for i, param := range params {
		if param.Key == key {
			params[i].Value = value
			return g
		}
	}
	g.gctx.AddParam(key, value)
	return g
}

func (g *GinTester) WithKV(key string, value any) *GinTester {
	g.gctx.Set(key, value)
	return g
}

func (g *GinTester) WithBody(t *testing.T, body any) *GinTester {
	b, err := json.Marshal(body)
	require.Nil(t, err)
	g.gctx.Request.Body = io.NopCloser(bytes.NewBuffer(b))
	return g
}

func (g *GinTester) WithMultipartForm(mf *multipart.Form) *GinTester {
	g.gctx.Request.MultipartForm = mf
	return g
}

func (g *GinTester) WithQuery(key, value string) *GinTester {
	q := g.gctx.Request.URL.Query()
	q.Add(key, value)
	g.gctx.Request.URL.RawQuery = q.Encode()
	return g
}

func (g *GinTester) SetPath(path string) *GinTester {
	g.gctx.Request.URL.Path = path
	return g
}

func (g *GinTester) WithHeader(key, value string) *GinTester {
	h := g.gctx.Request.Header
	if h == nil {
		h = map[string][]string{}
	}
	h.Add(key, value)
	g.gctx.Request.Header = h
	return g
}

func (g *GinTester) AddPagination(page int, per int) *GinTester {
	g.WithQuery("page", cast.ToString(page))
	g.WithQuery("per", cast.ToString(per))
	return g
}

func (g *GinTester) ResponseEq(t *testing.T, code int, msg string, expected any) {
	if !g._executed {
		require.FailNow(t, "call Execute method first")
	}
	var r = struct {
		Msg  string `json:"msg"`
		Data any    `json:"data,omitempty"`
	}{
		Msg:  msg,
		Data: expected,
	}
	b, err := json.Marshal(r)
	require.NoError(t, err)
	require.Equal(t, code, g.response.Code, g.response.Body.String())
	require.JSONEq(t, string(b), g.response.Body.String())

}

func (g *GinTester) ResponseEqSimple(t *testing.T, code int, expected any) {
	if !g._executed {
		require.FailNow(t, "call Execute method first")
	}
	b, err := json.Marshal(expected)
	require.NoError(t, err)
	require.Equal(t, code, g.response.Code, g.response.Body.String())
	require.JSONEq(t, string(b), g.response.Body.String())

}

func (g *GinTester) RequireUser(t *testing.T) {
	// use a tmp ctx to test no user case
	tmp := NewGinTester()
	tmp.gctx.Params = g.gctx.Params
	g.ginHandler(tmp.gctx)
	tmp._executed = true
	tmp.ResponseEq(t, http.StatusUnauthorized, component.ErrUserNotFound.Error(), nil)
	// add user to original test ctx now
	_ = g.WithUser()

}
