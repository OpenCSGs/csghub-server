package testinfra

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cast"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.temporal.io/server/temporaltest"
	"google.golang.org/protobuf/types/known/durationpb"
	"opencsg.com/csghub-server/api/httpbase"
	api_router "opencsg.com/csghub-server/api/router"
	api_workflow "opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
	dv_router "opencsg.com/csghub-server/dataviewer/router"
	user_router "opencsg.com/csghub-server/user/router"
	user_workflow "opencsg.com/csghub-server/user/workflow"
)

var chMu sync.Mutex

func chProjectRoot() {
	chMu.Lock()
	defer chMu.Unlock()
	for {
		_, err := os.Stat("builder/store/database/migrations")
		if err != nil {
			err = os.Chdir("../")
			if err != nil {
				panic(err)
			}
			continue
		}
		return
	}
}

type TestEnv struct {
	userStore           database.UserStore
	accessTokenStore    database.AccessTokenStore
	temporalServer      *temporaltest.TestServer
	userServer          *httpbase.GracefulServer
	datasetViewerServer *httpbase.GracefulServer
	apiServer           *httpbase.GracefulServer
}

func (t *TestEnv) Shutdown(ctx context.Context) error {
	var err error
	if t.temporalServer != nil {
		t.temporalServer.Stop()
	}
	if t.userServer != nil {
		err = errors.Join(err, t.userServer.Shutdown(ctx))
	}
	if t.datasetViewerServer != nil {
		err = errors.Join(err, t.datasetViewerServer.Shutdown(ctx))
	}
	if t.apiServer != nil {
		err = errors.Join(err, t.apiServer.Shutdown(ctx))
	}
	return err
}

func (t *TestEnv) CreateAccessToken(ctx context.Context, userName string, app types.AccessTokenApp) (string, error) {
	uw, err := t.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return "", err
	}
	token := cast.ToString(time.Now().UnixNano())
	err = t.accessTokenStore.Create(ctx, &database.AccessToken{
		Token:       token,
		User:        &uw,
		UserID:      uw.ID,
		IsActive:    true,
		ExpiredAt:   time.Now().Add(10 * time.Hour),
		Application: app,
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Create a new user in database and return an access token
func (t *TestEnv) CreateUser(ctx context.Context, userName string) (string, error) {
	namespace := &database.Namespace{
		Path: userName,
	}
	user := &database.User{
		Username:      userName,
		NickName:      userName,
		Email:         userName + "@csg.com",
		UUID:          userName + "uuid",
		RegProvider:   "casdoor",
		EmailVerified: true,
		RoleMask:      "user",
	}
	err := t.userStore.Create(ctx, user, namespace)
	if err != nil {
		return "", err
	}
	token, err := t.CreateAccessToken(ctx, userName, types.AccessTokenAppCSGHub)
	if err != nil {
		return "", err
	}
	return token, nil
}

func StartTestEnv() (*TestEnv, error) {
	env := &TestEnv{}
	chProjectRoot()
	ctx := context.TODO()
	config.SetConfigFile("common/config/test.toml")
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	// create test postgres container
	_, dsn := tests.CreateTestDB("csghub_integration_test" + cast.ToString(time.Now().Unix()))
	cfg.Database.DSN = dsn
	dbConfig := database.DBConfig{
		Dialect: database.DatabaseDialect(cfg.Database.Driver),
		DSN:     cfg.Database.DSN + "sslmode=disable",
	}
	// init db from test container
	database.InitDB(dbConfig)
	env.userStore = database.NewUserStoreWithDB(database.GetDB())
	env.accessTokenStore = database.NewAccessTokenStoreWithDB(database.GetDB())

	// create test gitaly
	configFile := "tests/gitaly.toml"
	// http://host.docker.internal:9091 is not accessible in github CI,
	// use http://172.17.0.1:9091
	if os.Getenv("GITHUB") == "true" {
		configFile = "tests/gitaly_github.toml"
	}
	req := testcontainers.ContainerRequest{
		Name:         "csghub_test_gitaly",
		Image:        "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/gitaly:v17.5.0",
		ExposedPorts: []string{"9876/tcp"},
		User:         "root",
		Env:          map[string]string{"GITALY_CONFIG_FILE": "/etc/gitaly.toml"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      configFile,
				ContainerFilePath: "/etc/gitaly.toml",
				FileMode:          0o700,
			},
		},
		Cmd: []string{"bash", "-c", "mkdir -p /home/git/repositories && rm -rf /srv/gitlab-shell/hooks/* && touch /srv/gitlab-shell/.gitlab_shell_secret && exec /scripts/process-wrapper"},
	}
	gitalyContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            true,
	})
	if err != nil {
		return env, err
	}
	time.Sleep(1 * time.Second)
	gitalyLocalPort, err := gitalyContainer.MappedPort(ctx, "9876")
	if err != nil {
		return env, err
	}
	tmp := strings.Split(string(gitalyLocalPort), "/")
	cfg.GitalyServer.Address = strings.ReplaceAll(cfg.GitalyServer.Address, ":9876", ":"+tmp[0])

	// create local temporal
	env.temporalServer = temporaltest.NewServer()
	cfg.WorkFLow.Endpoint = env.temporalServer.GetFrontendHostPort()

	nsclient, err := client.NewNamespaceClient(client.Options{HostPort: cfg.WorkFLow.Endpoint})
	if err != nil {
		return env, err
	}
	err = nsclient.Register(ctx, &workflowservice.RegisterNamespaceRequest{
		Namespace:                        "default",
		WorkflowExecutionRetentionPeriod: &durationpb.Duration{Seconds: 1000000000},
	})
	if err != nil {
		return env, err
	}

	// start redis
	req = testcontainers.ContainerRequest{
		Image:        "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/redis:7.2.5",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	rc, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return env, err
	}
	redisLocalPort, err := rc.MappedPort(ctx, "6379")
	if err != nil {
		return env, err
	}
	cfg.Redis.Endpoint = strings.ReplaceAll(cfg.Redis.Endpoint, "6379", redisLocalPort.Port())

	// start user service
	err = user_workflow.StartWorker(cfg)
	if err != nil {
		return env, err
	}
	ur, err := user_router.NewRouter(cfg)
	if err != nil {
		return env, err
	}
	env.userServer = httpbase.NewGracefulServer(
		httpbase.GraceServerOpt{
			Port: cfg.User.Port,
		},
		ur,
	)
	go env.userServer.Run()

	// start dataset viewer service
	client, err := temporal.NewClient(client.Options{
		HostPort: cfg.WorkFLow.Endpoint,
		Logger:   log.NewStructuredLogger(slog.Default()),
	}, "dataset-viewer")
	if err != nil {
		return env, err
	}
	dr, err := dv_router.NewDataViewerRouter(cfg, client)
	if err != nil {
		return env, err
	}
	env.datasetViewerServer = httpbase.NewGracefulServer(
		httpbase.GraceServerOpt{
			Port: cfg.DataViewer.Port,
		},
		dr,
	)
	go env.datasetViewerServer.Run()

	// start api
	as, err := api_router.NewServer(cfg, false)
	if err != nil {
		return env, err
	}
	env.apiServer = as
	go env.apiServer.Run()

	err = api_workflow.StartWorkflow(cfg)
	if err != nil {
		return env, err
	}
	var success bool
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		resp, err := http.Get("http://localhost:9091/api/v1/models?page=1&per=1")
		if err != nil {
			fmt.Println("health check failed, retry in 1 second")
			continue
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				fmt.Println("health check failed, retry in 1 second")
				continue
			}
			success = true
			break
		}
	}
	if !success {
		return env, errors.New("api health check failed")
	}
	fmt.Println("===== api health check success")
	return env, nil
}

func StartTestServer(t *testing.T) {
	ctx := context.Background()
	env, err := StartTestEnv()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := env.Shutdown(ctx); err != nil {
			t.Error("Timed out waiting on server to shut down")
		}
	})
}

type AddHeaderTransport struct {
	t       http.RoundTripper
	headers map[string]string
}

func (adt *AddHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range adt.headers {
		req.Header.Add(k, v)
	}
	return adt.t.RoundTrip(req)
}

func GetClient(accessToken string) *http.Client {
	header := map[string]string{}
	if accessToken != "" {
		header["Authorization"] = fmt.Sprintf("Bearer %s", accessToken)
	}
	return &http.Client{Transport: &AddHeaderTransport{
		t:       http.DefaultTransport,
		headers: header,
	}}
}
