package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestDeleteXnetMessagesByFilterWithWorkQueueConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/csghub_nats:2.10.16",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForLog("Server is ready").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		require.NoError(t, testcontainers.TerminateContainer(container, testcontainers.StopContext(cleanupCtx)))
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, nat.Port("4222/tcp"))
	require.NoError(t, err)
	serverURL := fmt.Sprintf("nats://%s:%s", host, port.Port())
	var connection *nats.Conn
	require.Eventually(t, func() bool {
		candidate, connectErr := nats.Connect(serverURL, nats.Timeout(time.Second))
		if connectErr != nil {
			return false
		}
		connection = candidate
		return true
	}, 10*time.Second, 100*time.Millisecond, "NATS server did not accept connections")
	t.Cleanup(connection.Close)

	js, err := jetstream.New(connection)
	require.NoError(t, err)
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:      LfsXnetMigrateGroup.StreamName,
		Subjects:  []string{LfsMigrationSubject},
		Retention: jetstream.WorkQueuePolicy,
	})
	require.NoError(t, err)
	consumer, err := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          LfsXnetMigrateGroup.ConsumerName,
		Durable:       LfsXnetMigrateGroup.ConsumerName,
		FilterSubject: LfsMigrationSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	require.NoError(t, err)

	for _, payload := range []string{
		`{"repo_id":"target","file_sha256":"one"}`,
		`{"repo_id":"other","file_sha256":"two"}`,
	} {
		_, err = js.Publish(ctx, LfsMigrationSubject, []byte(payload))
		require.NoError(t, err)
	}

	queue := &Nats{conn: connection, js: js}
	err = queue.DeleteMessagesByFilter(LfsXnetMigrateGroup.StreamName, func(data []byte) bool {
		var message struct {
			RepoID string `json:"repo_id"`
		}
		return json.Unmarshal(data, &message) == nil && message.RepoID == "target"
	})
	require.NoError(t, err)

	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
	require.NoError(t, err)
	var remaining jetstream.Msg
	for msg := range batch.Messages() {
		remaining = msg
	}
	require.NoError(t, batch.Error())
	require.NotNil(t, remaining)
	require.JSONEq(t, `{"repo_id":"other","file_sha256":"two"}`, string(remaining.Data()))
	require.NoError(t, remaining.Ack())

	_, err = stream.Consumer(ctx, LfsXnetMigrateGroup.ConsumerName)
	require.NoError(t, err, "the existing durable consumer must remain intact")
}
