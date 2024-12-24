package utils

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParameters_GetSceneFromContext(t *testing.T) {
	t.Run("valid scene value", func(t *testing.T) {
		values := url.Values{}
		values.Add("scene", "2")
		req, err := http.NewRequest(http.MethodGet, "/test?"+values.Encode(), nil)
		require.Nil(t, err)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		res, err := GetSceneFromContext(ginContext)
		require.Nil(t, err)
		require.Equal(t, 2, res)
	})

	t.Run("invalid scene value", func(t *testing.T) {
		values := url.Values{}
		values.Add("scene", "a")
		req, err := http.NewRequest(http.MethodGet, "/test?"+values.Encode(), nil)
		require.Nil(t, err)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		res, err := GetSceneFromContext(ginContext)
		require.NotNil(t, err)
		require.Equal(t, 0, res)
	})
}
