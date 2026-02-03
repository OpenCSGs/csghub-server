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

func (n *Nats) PurgeStream(streamName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := n.js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("[nats] failed to get stream %s error: %w", streamName, err)
	}

	err = stream.Purge(ctx)
	if err != nil {
		return fmt.Errorf("[nats] failed to purge stream %s error: %w", streamName, err)
	}

	slog.Info("[nats] stream purged successfully", slog.String("stream", streamName))
	return nil
}

func (n *Nats) DeleteMessagesByFilter(streamName string, filter func(data []byte) bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := n.js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("[nats] failed to get stream %s error: %w", streamName, err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return fmt.Errorf("[nats] failed to get stream info for %s error: %w", streamName, err)
	}

	if info.State.Msgs == 0 {
		slog.Info("[nats] no messages to delete in stream", slog.String("stream", streamName))
		return nil
	}

	tempConsumerName := fmt.Sprintf("temp-delete-consumer-%d", time.Now().UnixNano())
	consumer, err := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          tempConsumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return fmt.Errorf("[nats] failed to create temporary consumer error: %w", err)
	}
	defer func() {
		if delErr := stream.DeleteConsumer(ctx, tempConsumerName); delErr != nil {
			slog.Error("[nats] failed to delete temporary consumer",
				slog.String("consumer", tempConsumerName),
				slog.Any("error", delErr))
		}
	}()

	deletedCount := 0
	batch, err := consumer.Fetch(int(info.State.Msgs))
	if err != nil {
		return fmt.Errorf("[nats] failed to fetch messages error: %w", err)
	}

	for msg := range batch.Messages() {
		if filter(msg.Data()) {
			metadata, err := msg.Metadata()
			if err != nil {
				slog.Error("[nats] failed to get message metadata", slog.Any("error", err))
				continue
			}

			err = stream.DeleteMsg(ctx, metadata.Sequence.Stream)
			if err != nil {
				slog.Error("[nats] failed to delete message",
					slog.Uint64("sequence", metadata.Sequence.Stream),
					slog.Any("error", err))
				continue
			}
			deletedCount++
		}
		if err := msg.Ack(); err != nil {
			slog.Warn("[nats] failed to ack message", slog.Any("error", err))
		}
	}

	slog.Info("[nats] messages deleted by filter",
		slog.String("stream", streamName),
		slog.Int("deleted_count", deletedCount))
	return nil
}
