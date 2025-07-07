package factory

import (
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/notification/notifychannel"
)

type Factory interface {
	GetChannel(name string) (notifychannel.Notifier, error)
	RegisterChannel(name string, channel notifychannel.Notifier)
}

type factoryImpl struct {
	channels map[string]notifychannel.Notifier
}

func NewFactory(config *config.Config) Factory {
	factory := &factoryImpl{}
	factory.channels = make(map[string]notifychannel.Notifier)

	// initialize channels, and register them
	registerChannels(config, factory)
	return factory
}

func (f *factoryImpl) GetChannel(name string) (notifychannel.Notifier, error) {
	channel, ok := f.channels[name]
	if !ok {
		return nil, fmt.Errorf("channel %s not registered", name)
	}
	return channel, nil
}

func (f *factoryImpl) RegisterChannel(name string, channel notifychannel.Notifier) {
	slog.Info("register notify channel successfully", "channel", name)
	f.channels[name] = channel
}
