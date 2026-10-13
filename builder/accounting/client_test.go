package accounting

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/errorx"
)

func TestAccountingClientDoRequestParsesRemoteErrorCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = io.WriteString(w, `{"code":"ACT-ERR-0","msg":"ACT-ERR-0"}`)
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	require.NoError(t, err)

	ac := &accountingClientImpl{remote: parsed, client: http.DefaultClient}
	_, err = ac.handleResponse(ac.doRequest(http.MethodPost, "/token-plans/ns-1/purchase?current_user=u", nil))
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrInsufficientBalance))

	var customErr errorx.CustomError
	require.True(t, errors.As(err, &customErr))
	require.Equal(t, "ACT-ERR-0", customErr.Code())
}

func TestAccountingClientDoRequestParsesTokenPlanErrorCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"code":"TOKENPLAN-ERR-1","msg":"TOKENPLAN-ERR-1"}`)
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	require.NoError(t, err)

	ac := &accountingClientImpl{remote: parsed, client: http.DefaultClient}
	_, err = ac.handleResponse(ac.doRequest(http.MethodPut, "/token-plans/ns-1/cancel?current_user=u", nil))
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrTokenPlanInUsing))

	var customErr errorx.CustomError
	require.True(t, errors.As(err, &customErr))
	require.Equal(t, "TOKENPLAN-ERR-1", customErr.Code())
}

func TestAccountingClientDoRequestKeepsUnknownErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"code":"invalid","msg":"boom"}`)
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	require.NoError(t, err)

	ac := &accountingClientImpl{remote: parsed, client: http.DefaultClient}
	_, err = ac.handleResponse(ac.doRequest(http.MethodGet, "/token-plans/ns-1/list?current_user=u", nil))
	require.ErrorContains(t, err, "unexpected http status and error")
	require.ErrorContains(t, err, "invalid")
}
