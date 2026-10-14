package scenariomgr

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
)

func TestProcessMessageCopiesScenarioMessageID(t *testing.T) {
	scenario := types.MessageScenario("test-pass-msg-uuid")
	t.Cleanup(func() {
		delete(scenarioRegistry, scenario)
	})
	RegisterScenario(scenario, &ScenarioDefinition{
		Channels: []types.MessageChannel{types.MessageChannelEmail},
	})

	recorder := &recordingNotifier{}
	redisClient := mockcache.NewMockRedisClient(t)
	redisClient.EXPECT().Get(context.Background(), "notification:retry_message:msg-uuid-1").Return("", redis.Nil)
	redisClient.EXPECT().Del(context.Background(), "notification:retry_message:msg-uuid-1").Return(nil)

	processor := &MessageProcessor{
		config:               &config.Config{},
		notifyChannelFactory: &stubChannelFactory{channel: recorder},
		redis:                redisClient,
	}

	err := processor.ProcessMessage(context.Background(), types.ScenarioMessage{
		MsgUUID:    "msg-uuid-1",
		Scenario:   scenario,
		Parameters: `{}`,
		Priority:   types.MessagePriorityHigh,
	})

	require.NoError(t, err)
	require.NotNil(t, recorder.req)
	require.Equal(t, "msg-uuid-1", recorder.req.MsgUUID)
	require.Equal(t, types.MessagePriorityHigh, recorder.req.Priority)
}

type recordingNotifier struct {
	req *notifychannel.NotifyRequest
}

func (n *recordingNotifier) IsFormatRequired() bool {
	return false
}

func (n *recordingNotifier) Send(_ context.Context, req *notifychannel.NotifyRequest) error {
	copied := *req
	n.req = &copied
	return nil
}

type stubChannelFactory struct {
	channel notifychannel.Notifier
}

func (f *stubChannelFactory) GetChannel(string) (notifychannel.Notifier, error) {
	return f.channel, nil
}

func (f *stubChannelFactory) RegisterChannel(string, notifychannel.Notifier) {}
