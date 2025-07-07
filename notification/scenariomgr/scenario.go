package scenariomgr

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
)

type NotificationData struct {
	MessageData any
	Receiver    *notifychannel.Receiver
}

type GetDataFunc func(ctx context.Context, conf *config.Config, msg types.ScenarioMessage) (*NotificationData, error)

type ScenarioDefinition struct {
	// channels to send notification
	Channels []types.MessageChannel
	// optional, if not set, use msg.Parameters to unmarshal data
	DefaultGetDataFunc GetDataFunc
	// optional, if not set, use DefaultGetDataFunc
	ChannelGetDataFunc map[types.MessageChannel]GetDataFunc
}

var scenarioRegistry = make(map[types.MessageScenario]*ScenarioDefinition)

func RegisterScenario(scenario types.MessageScenario, def *ScenarioDefinition) {
	if _, ok := scenarioRegistry[scenario]; ok {
		slog.Info("scenario already registered, skip", "scenario", scenario)
		return
	}
	scenarioRegistry[scenario] = def
	slog.Info("scenario registered", "scenario", scenario, "channels", def.Channels)
}

func GetScenario(scenario types.MessageScenario) (*ScenarioDefinition, bool) {
	def, ok := scenarioRegistry[scenario]
	return def, ok
}
