package api_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"opencsg.com/csghub-server/tests/testinfra"
)

func TestIntegrationModel_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.TODO()
	env, err := testinfra.StartTestEnv()
	defer func() { _ = env.Shutdown(ctx) }()
	require.NoError(t, err)
	token, err := env.CreateUser(ctx, "user1")
	require.NoError(t, err)
	userClientA := testinfra.GetClient(token)
	token, err = env.CreateUser(ctx, "user2")
	require.NoError(t, err)
	userClientB := testinfra.GetClient(token)
	anonymousClient := testinfra.GetClient("")

	type triResponse struct {
		codes []int
		bodys [][]byte
	}
	tripleDo := func(method string, url string, body string) *triResponse {
		rp := &triResponse{}
		for _, client := range []*http.Client{anonymousClient, userClientA, userClientB} {
			buf := bytes.NewBuffer([]byte(body))
			req, err := http.NewRequest(method, url, buf)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			rp.codes = append(rp.codes, resp.StatusCode)
			rp.bodys = append(rp.bodys, bodyBytes)
		}
		return rp
	}

	// create model anonymous
	data := `{"name":"test1","nickname":"","namespace":"user1","license":"apache-2.0","description":"","private":false}`
	req, err := http.NewRequest(
		"POST", "http://localhost:9091/api/v1/models", bytes.NewBuffer([]byte(data)),
	)
	require.NoError(t, err)
	resp, err := anonymousClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 401, resp.StatusCode)

	// create model login
	req, err = http.NewRequest(
		"POST", "http://localhost:9091/api/v1/models", bytes.NewBuffer([]byte(data)),
	)
	require.NoError(t, err)
	resp, err = userClientA.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)

	// get models, all 3 clients should be able to see new model
	rp := tripleDo(
		"GET", "http://localhost:9091/api/v1/models?page=1&per=16&search=&sort=trending", "",
	)
	require.Equal(t, []int{200, 200, 200}, rp.codes)
	for _, b := range rp.bodys {
		require.Equal(t, int64(1), gjson.GetBytes(b, "total").Int())
		require.Equal(t, "test1", gjson.GetBytes(b, "data.0.name").String())
	}

	// get model detail, all 3 clients should be able to access
	rp = tripleDo(
		"GET", "http://localhost:9091/api/v1/models/user1/test1", "",
	)
	require.Equal(t, []int{200, 200, 200}, rp.codes)
	for _, b := range rp.bodys {
		require.Equal(t, "test1", gjson.GetBytes(b, "data.name").String())
	}

	// create private model
	data = `{"name":"test2","nickname":"","namespace":"user1","license":"apache-2.0","description":"","private":true}`
	req, err = http.NewRequest(
		"POST", "http://localhost:9091/api/v1/models", bytes.NewBuffer([]byte(data)),
	)
	require.NoError(t, err)
	resp, err = userClientA.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)

	// get model, only user A can see the new model in list
	rp = tripleDo(
		"GET", "http://localhost:9091/api/v1/models?page=1&per=16&search=&sort=trending", "",
	)
	require.Equal(t, []int{200, 200, 200}, rp.codes)
	for i, b := range rp.bodys {
		if i == 1 {
			require.Equal(t, int64(2), gjson.GetBytes(b, "total").Int())
			require.Equal(t, "test2", gjson.GetBytes(b, "data.1.name").String())
		} else {
			require.Equal(t, int64(1), gjson.GetBytes(b, "total").Int())
			require.Equal(t, "test1", gjson.GetBytes(b, "data.0.name").String())
		}
	}

	// get model detail, only user A can access the new model
	rp = tripleDo(
		"GET", "http://localhost:9091/api/v1/models/user1/test2", "",
	)
	require.Equal(t, []int{403, 200, 403}, rp.codes)
	for i, b := range rp.bodys {
		if i == 1 {
			require.Equal(t, "test2", gjson.GetBytes(b, "data.name").String())
		}
	}

	// update model file, public
	rp = tripleDo(
		"PUT", "http://localhost:9091/api/v1/models/user1/test1/raw/README.md",
		`{"content":"Ci0tLQpsaWNlbnNlOiBnZW1tYQotLS0Keg==","message":"Update README.md","branch":"main","new_branch":"main","sha":"4d1cb859ec3b14226026a965517d0e0c07c9883e"}`,
	)
	require.Equal(t, []int{401, 200, 500}, rp.codes)

	// update model file, private
	rp = tripleDo(
		"PUT", "http://localhost:9091/api/v1/models/user1/test2/raw/README.md",
		`{"content":"Ci0tLQpsaWNlbnNlOiBnZW1tYQotLS0Keg==","message":"Update README.md","branch":"main","new_branch":"main","sha":"4d1cb859ec3b14226026a965517d0e0c07c9883e"}`,
	)
	require.Equal(t, []int{401, 200, 500}, rp.codes)

	// delete model, public
	rp = tripleDo("DELETE", "http://localhost:9091/api/v1/models/user1/test1", "")
	require.Equal(t, []int{401, 200, 500}, rp.codes)

	// delete model, private
	rp = tripleDo("DELETE", "http://localhost:9091/api/v1/models/user1/test2", "")
	require.Equal(t, []int{401, 200, 500}, rp.codes)

	// model list empty
	rp = tripleDo(
		"GET", "http://localhost:9091/api/v1/models?page=1&per=16&search=&sort=trending", "",
	)
	require.Equal(t, []int{200, 200, 200}, rp.codes)
	for _, b := range rp.bodys {
		require.Equal(t, int64(0), gjson.GetBytes(b, "total").Int())
	}

}
