package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func Test_XOpenCSGHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add("X-OPENCSG-S3-Internal", "true")

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Request = req
	if _, ok := ginContext.Get("X-OPENCSG-S3-Internal"); ok {
		t.Errorf("X-OPENCSG-S3-Internal should not exist")
	}

	XOpenCSGHeader()(ginContext)

	if v, ok := ginContext.Get("X-OPENCSG-S3-Internal"); !ok || v != true {
		t.Errorf("X-OPENCSG-S3-Internal should be true")
	}

}
