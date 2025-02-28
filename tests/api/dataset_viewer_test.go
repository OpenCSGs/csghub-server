package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/tests/testinfra"
)

func TestIntegrationDatasetViewer_Workflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.TODO()
	env, err := testinfra.StartTestEnv()
	defer func() { _ = env.Shutdown(ctx) }()
	require.NoError(t, err)
	token, err := env.CreateUser(ctx, "user")
	require.NoError(t, err)
	userClient := testinfra.GetClient(token)

	data := `{"name":"test","nickname":"","namespace":"user","license":"apache-2.0","description":"","private":false}`
	req, err := http.NewRequest(
		"POST", "http://localhost:9091/api/v1/datasets", bytes.NewBuffer([]byte(data)),
	)
	require.NoError(t, err)
	resp, err := userClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	resp, err = userClient.Get("http://localhost:9091/api/v1/datasets/user/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var model types.Model
	err = json.Unmarshal([]byte(gjson.GetBytes(body, "data").Raw), &model)
	require.NoError(t, err)
	cloneURL := model.Repository.HTTPCloneURL

	token, err = env.CreateAccessToken(ctx, "user", types.AccessTokenAppGit)
	require.NoError(t, err)
	url := strings.ReplaceAll(cloneURL, "http://", fmt.Sprintf("http://%s:%s@", "user", token))
	dir := "dataset_clone"
	err = gitClone(url, dir)
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	// add yaml config to readme
	file, err := os.OpenFile(dir+"/README.md", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	defer file.Close()
	fileContent := ""
	buf := make([]byte, 1024)
	for {
		n, err := file.Read(buf)
		if err != nil {
			break
		}
		fileContent += string(buf[:n])
	}
	configContent := `---
configs:
- config_name: defaultgo
  data_files:
  - split: traingo
    path: "train/0.parquet"
  - split: testgo
    path: "test/1.parquet"
---
`
	newContent := configContent + fileContent
	_, err = file.Seek(0, 0)
	require.NoError(t, err)
	_, err = file.WriteString(newContent)
	require.NoError(t, err)

	err = exec.Command("mkdir", dir+"/train").Run()
	require.NoError(t, err)
	err = exec.Command("mkdir", dir+"/test").Run()
	require.NoError(t, err)
	err = exec.Command("cp", "tests/0.parquet", dir+"/train/0.parquet").Run()
	require.NoError(t, err)
	err = exec.Command("cp", "tests/1.parquet", dir+"/test/1.parquet").Run()
	require.NoError(t, err)
	err = gitCommitAndPush(dir)
	require.NoError(t, err)

	resp, err = userClient.Get("http://localhost:9091/api/v1/datasets/user/test/dataviewer/catalog")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	expected := `{"msg":"OK","data":{"configs":[{"config_name":"defaultgo","data_files":[{"split":"traingo","path":["train/0.parquet"]},{"split":"testgo","path":["test/1.parquet"]}]}],"dataset_info":[{"config_name":"defaultgo","splits":[{"name":"traingo","num_examples":20},{"name":"testgo","num_examples":20}]}],"status":0,"logs":""}}
`
	require.Equal(t, expected, string(body))

	// check auto created branch
	time.Sleep(2 * time.Second)
	resp, err = userClient.Get("http://localhost:9091/api/v1/datasets/user/test/branches")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	branches := gjson.GetBytes(body, "data.#.name").String()
	require.Equal(t, `["main","refs-convert-parquet"]`, branches)
}
