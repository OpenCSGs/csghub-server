package mq

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/common/config"
)

var _ MessageQueue = (*Nats)(nil)

type Nats struct {
	conn *nats.Conn
	js   jetstream.JetStream
}

func NewNats(cfg *config.Config) (MessageQueue, error) {
	nc, err := nats.Connect(
		cfg.Nats.URL,
		nats.Timeout(2*time.Second),
		nats.ReconnectWait(10*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("[nats] failed to connect to server: %w", err)
	}
	jetstream, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("[nats] failed to new jetstream instance context: %w", err)
	}
	return &Nats{
		conn: nc,
		js:   jetstream,
	}, nil
}

func (n *Nats) Publish(topic string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := n.js.Publish(ctx, topic, data)
	if err != nil {
		return fmt.Errorf("[nats] failed to publish message to subject %s error: %w", topic, err)
	}
	return nil
}

func (n *Nats) Subscribe(params SubscribeParams) error {
	jsc, err := n.getOrCreateStreamConsumer(params)
	if err != nil {
		return fmt.Errorf("[nats] failed to verify or create stream %s and consumer %s error: %w",
			params.Group.StreamName, params.Group.ConsumerName, err)
	}

	_, err = jsc.Consume(func(msg jetstream.Msg) {
		err := params.Callback(
			msg.Data(),
			MessageMeta{
				Topic: msg.Subject(),
			},
		)

		if err != nil {
			slog.Error("[nats] failed to invoke callback for message", slog.Any("topics", params.Topics), slog.Any("error", err))
		}

		if params.AutoACK {
			action := ""
			if err != nil && params.IsRedeliverForCBFailed {
				err = msg.Nak()
				action = "nak"
			} else {
				err = msg.Ack()
				action = "ack"
			}
			if err != nil {
				slog.Error(fmt.Sprintf("[nats] failed to %s message", action),
					slog.Any("topics", params.Topics), slog.Any("error", err))
			}
		}
	})
	if err != nil {
		return fmt.Errorf("[nats] failed to subscribe to topics %s error: %w", params.Topics, err)
	}
	return nil
}

func (n *Nats) getOrCreateStreamConsumer(params SubscribeParams) (jetstream.Consumer, error) {
	var (
		err error
		jss jetstream.Stream
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	jss, err = n.js.CreateOrUpdateStream(ctx,
		jetstream.StreamConfig{
			Name:         params.Group.StreamName,
			Subjects:     params.Topics,
			MaxConsumers: -1,
			MaxMsgs:      -1,
			MaxBytes:     -1,
		})
	if err != nil {
		return nil, fmt.Errorf("[nats] failed to create or update stream %s error: %w", params.Group.StreamName, err)
	}

	jsc, err := jss.CreateOrUpdateConsumer(ctx,
		jetstream.ConsumerConfig{
			Name:           params.Group.ConsumerName,
			Durable:        params.Group.ConsumerName,
			FilterSubjects: params.Topics,
			AckPolicy:      jetstream.AckExplicitPolicy,
			DeliverPolicy:  jetstream.DeliverAllPolicy,
		})
	if err != nil {
		return nil, fmt.Errorf("[nats] failed to create or update consumer %s error: %w", params.Group.ConsumerName, err)
	}

	return jsc, nil
}
