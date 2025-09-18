package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// mock config
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	os.Exit(m.Run())
}

func TestNewHttpClient(t *testing.T) {
	client := NewHttpClient("http://test.com")
	assert.NotNil(t, client)
	assert.NotNil(t, client.logger)
	assert.Equal(t, uint(1), client.retry)
}

func TestHttpClient_WithRetry(t *testing.T) {
	client := NewHttpClient("http://test.com")
	client.WithRetry(5)
	assert.Equal(t, uint(5), client.retry)
}

func TestHttpClient_Get(t *testing.T) {
	httpmock.RegisterResponder("GET", "http://test.com/api/v1/test",
		httpmock.NewStringResponder(200, `{"message": "success"}`))

	client := NewHttpClient("http://test.com")
	var result map[string]interface{}
	err := client.Get(context.Background(), "/api/v1/test", &result)

	assert.NoError(t, err)
	assert.Equal(t, "success", result["message"])
}

func TestHttpClient_Get_WithRetry(t *testing.T) {
	httpmock.Reset() // Add this line to reset call count before the test
	responder := httpmock.NewErrorResponder(errors.New("network error"))
	httpmock.RegisterResponder("GET", "http://test.com/api/v1/retry", responder)

	client := NewHttpClient("http://test.com").WithRetry(3)
	var result map[string]interface{}
	err := client.Get(context.Background(), "/api/v1/retry", &result)

	assert.Error(t, err)
	// 1 initial attempt + 2 retries = 3 calls
	assert.Equal(t, 3, httpmock.GetTotalCallCount())
}

func TestHttpClient_Post(t *testing.T) {
	httpmock.RegisterResponder("POST", "http://test.com/api/v1/test",
		func(req *http.Request) (*http.Response, error) {
			var reqBody map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			assert.Equal(t, "value", reqBody["key"])
			resp, err := httpmock.NewJsonResponse(200, map[string]interface{}{"status": "created"})
			return resp, err
		},
	)

	client := NewHttpClient("http://test.com")
	requestData := map[string]interface{}{"key": "value"}
	var result map[string]interface{}
	err := client.Post(context.Background(), "/api/v1/test", requestData, &result)

	assert.NoError(t, err)
	assert.Equal(t, "created", result["status"])
}

func TestHttpClient_PostResponse(t *testing.T) {
	// Create a mock server
	httpmock.RegisterResponder("POST", "http://test.com/api/v1/post-resp",
		httpmock.NewStringResponder(http.StatusCreated, "created"))

	// Create an instance of HttpClient
	client := NewHttpClient("http://test.com")

	// Define the request data
	requestData := map[string]string{"key": "value"}

	// Call the PostResponse method
	resp, err := client.PostResponse(context.Background(), "/api/v1/post-resp", requestData)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check the response
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}
