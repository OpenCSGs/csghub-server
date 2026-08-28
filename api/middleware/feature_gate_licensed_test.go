//go:build ee || (saas && !license_issuer)

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type fakeLicensePolicy struct {
	featureErr error
	calls      int
}

func (f *fakeLicensePolicy) CheckFeatureEnabled(context.Context, types.FeatureDefinition) error {
	f.calls++
	return f.featureErr
}

var _ component.LicensePolicyComponent = (*fakeLicensePolicy)(nil)

type mutableLicenseStatusReader struct {
	status *types.LicenseStatusResp
	err    error
}

func (r *mutableLicenseStatusReader) GetLicenseStatusInternal(context.Context) (*types.LicenseStatusResp, error) {
	return r.status, r.err
}

func TestFeatureGateMiddleware_AllowsEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy := &fakeLicensePolicy{}

	router := gin.New()
	router.Use(FeatureGate(policy, types.FeatureAuditLog))
	router.GET("/clusters", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/clusters", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, policy.calls)
}

func TestFeatureGateMiddleware_BlocksDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy := &fakeLicensePolicy{featureErr: errorx.ErrLicenseFeatureDisabled}

	router := gin.New()
	router.Use(FeatureGate(policy, types.FeatureAuditLog))
	router.GET("/clusters", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/clusters", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), errorx.ErrLicenseFeatureDisabled.Error())
}

func TestFeatureGateMiddleware_ServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy := &fakeLicensePolicy{featureErr: errors.New("policy failed")}

	router := gin.New()
	router.Use(FeatureGate(policy, types.FeatureAuditLog))
	router.GET("/clusters", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/clusters", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFeatureGateMiddleware_LicenseToHTTPWiring(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reader := &mutableLicenseStatusReader{}
	policy, err := component.NewLicensePolicyComponentWithReader(nil, reader)
	require.NoError(t, err)

	router := gin.New()
	router.Use(FeatureGate(policy, types.FeatureAuditLog))
	router.GET("/audit_logs", func(c *gin.Context) { c.Status(http.StatusOK) })

	tests := []struct {
		name       string
		status     *types.LicenseStatusResp
		err        error
		wantStatus int
	}{
		{name: "explicit true", status: &types.LicenseStatusResp{DataBody: types.DataBody{Extra: `{"features":{"feature.audit_log":true}}`}}, wantStatus: http.StatusOK},
		{name: "explicit false", status: &types.LicenseStatusResp{DataBody: types.DataBody{Extra: `{"features":{"feature.audit_log":false}}`}}, wantStatus: http.StatusForbidden},
		{name: "legacy missing key", status: &types.LicenseStatusResp{DataBody: types.DataBody{Extra: `{}`}}, wantStatus: http.StatusOK},
		{name: "no license", wantStatus: http.StatusForbidden},
		{name: "reader error", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader.status = tt.status
			reader.err = tt.err
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/audit_logs", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
