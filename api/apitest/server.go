package apitest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/api/router"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type ResponseHelper struct {
	response *httptest.ResponseRecorder
}

func (h *ResponseHelper) Response() *httptest.ResponseRecorder {
	return h.response
}

func (h *ResponseHelper) ResponseEq(t *testing.T, code int, msg string, expected any) {
	var r = struct {
		Msg  string `json:"msg"`
		Data any    `json:"data,omitempty"`
	}{
		Msg:  msg,
		Data: expected,
	}
	b, err := json.Marshal(r)
	require.NoError(t, err)
	require.Equal(t, code, h.response.Code, h.response.Body.String())
	require.JSONEq(t, string(b), h.response.Body.String())

}

func (h *ResponseHelper) ResponseEqSimple(t *testing.T, code int, expected any) {
	b, err := json.Marshal(expected)
	require.NoError(t, err)
	require.Equal(t, code, h.response.Code, h.response.Body.String())
	require.JSONEq(t, string(b), h.response.Body.String())

}

type TestServer struct {
	server *router.ServerImpl
}

func NewTestServer(t *testing.T, option func(s *router.ServerImpl)) *TestServer {
	gin.SetMode(gin.ReleaseMode)
	mu := mockcomponent.NewMockUserComponent(t)
	mu.EXPECT().FindByAccessToken(mock.Anything, "u:p").Return(&database.User{
		Username: "u",
	}, nil).Maybe()
	mm := mockcomponent.NewMockMirrorComponent(t)
	mm.EXPECT().FindWithMapping(
		mock.Anything, mock.Anything, "u", "r", types.HFMapping,
	).Return(&database.Repository{
		Path: "u/r",
	}, nil).Maybe()
	config := &config.Config{}
	config.GitServer.Type = types.GitServerTypeGitaly
	now := time.Now()
	md := middleware.NewMiddlewareDI(config, mu, mm, nil)
	server := &router.ServerImpl{
		BaseServer: &router.BaseServer{
			Middleware: md,
			Config:     config,
		},
	}
	option(server)
	err := server.RegisterRoutes(false)
	if err != nil {
		panic(err)
	}
	fmt.Println("====x", time.Since(now))
	return &TestServer{server: server}
}

func (ts *TestServer) NewRequest(method, url string, body any) (*http.Request, error) {
	d, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return http.NewRequest(method, url, strings.NewReader(string(d)))
}

func (ts *TestServer) NewGetRequest(url string) (*http.Request, error) {
	return http.NewRequest(http.MethodGet, url, nil)
}

func (ts *TestServer) NewPostRequest(url string, body any) (*http.Request, error) {
	d, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return http.NewRequest(http.MethodPost, url, strings.NewReader(string(d)))
}

func (ts *TestServer) NewPutRequest(url string, body any) (*http.Request, error) {
	d, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return http.NewRequest(http.MethodPut, url, strings.NewReader(string(d)))
}

func (ts *TestServer) NewDeleteRequest(url string) (*http.Request, error) {
	return http.NewRequest(http.MethodDelete, url, nil)
}

func (ts *TestServer) AuthRequest(req *http.Request) *http.Request {
	req.Header.Add("Authorization", "Bearer u:p")
	return req
}

func (ts *TestServer) Send(req *http.Request) *ResponseHelper {
	w := httptest.NewRecorder()
	ts.server.Engine.ServeHTTP(w, req)
	return &ResponseHelper{response: w}
}

func (ts *TestServer) AuthAndSend(t *testing.T, req *http.Request) *ResponseHelper {
	r := ts.Send(req)
	require.Equal(t, 401, r.response.Code)

	ts.AuthRequest(req)
	w := httptest.NewRecorder()
	ts.server.Engine.ServeHTTP(w, req)
	return &ResponseHelper{response: w}
}
