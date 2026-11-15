//go:build gitaly_integration

package gitaly

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	gitalyauth "gitlab.com/gitlab-org/gitaly/v16/auth"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const mirrorEgressGitalyConfig = `listen_addr = "0.0.0.0:8075"
bin_dir = "/usr/local/bin"
[auth]
token = "test-token"
[git]
use_bundled_binaries = true
[[git.config]]
key = "http.proxy"
value = "http://127.0.0.1:8080"
[[storage]]
name = "default"
path = "/tmp/repositories"
[gitlab-shell]
dir = "/srv/gitlab-shell"
[gitlab]
url = "http://127.0.0.1:8081"
secret = "test-secret"
`

type mirrorEgressServer struct {
	client   gitalypb.RepositoryServiceClient
	statsURL string
}

func buildMirrorEgressFixture(t *testing.T) string {
	t.Helper()
	fixture := filepath.Join(t.TempDir(), "mirror-egress")
	build := exec.Command("go", "build", "-o", fixture, "./testdata/mirror-egress")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	return fixture
}

func startMirrorEgressServer(t *testing.T, ctx context.Context, fixture string, direct bool) mirrorEgressServer {
	t.Helper()
	start := "/usr/local/bin/gitaly /test/config.toml"
	if direct {
		start = "/bin/sh /test/direct-egress.sh " + start
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/gitaly:v16.4.6",
			User:  "root", ExposedPorts: []string{"8075/tcp", "8081/tcp"},
			Entrypoint: []string{"/bin/sh", "-c"},
			Cmd:        []string{"echo '127.0.0.1 trusted.invalid' >> /etc/hosts; mkdir -p /tmp/repositories; /test/fixture & exec " + start},
			Env:        map[string]string{"http_proxy": "http://127.0.0.1:8080", "https_proxy": "http://127.0.0.1:8080", "all_proxy": "http://127.0.0.1:8080", "NO_PROXY": "", "no_proxy": ""},
			Files: []testcontainers.ContainerFile{
				{HostFilePath: fixture, ContainerFilePath: "/test/fixture", FileMode: 0755},
				{HostFilePath: "../../../../docker/gitaly/direct-egress.sh", ContainerFilePath: "/test/direct-egress.sh", FileMode: 0644},
				{Reader: strings.NewReader(mirrorEgressGitalyConfig), ContainerFilePath: "/test/config.toml", FileMode: 0644},
			},
			WaitingFor: wait.ForAll(wait.ForListeningPort("8075/tcp"), wait.ForListeningPort("8081/tcp")).WithDeadline(time.Minute),
		}, Started: true,
	})
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if t.Failed() {
				logs, logErr := container.Logs(cleanupCtx)
				if logErr == nil {
					data, _ := io.ReadAll(logs)
					logs.Close()
					t.Log(string(data))
				}
			}
			_ = container.Terminate(cleanupCtx)
		})
	}
	require.NoError(t, err)
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "8075/tcp")
	require.NoError(t, err)
	conn, err := grpc.NewClient(net.JoinHostPort(host, port.Port()), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(gitalyauth.RPCCredentialsV2("test-token")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	statsPort, err := container.MappedPort(ctx, "8081/tcp")
	require.NoError(t, err)
	return mirrorEgressServer{client: gitalypb.NewRepositoryServiceClient(conn), statsURL: "http://" + net.JoinHostPort(host, statsPort.Port())}
}

func (s mirrorEgressServer) counts(t *testing.T) map[string]int64 {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{}}
	defer client.CloseIdleConnections()
	resp, err := client.Get(s.statsURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	var counts map[string]int64
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&counts))
	return counts
}
