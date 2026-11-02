package mq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/common/config"
)

var _ MessageQueue = (*Nats)(nil)

type Nats struct {
	conn                 *nats.Conn
	js                   jetstream.JetStream
	xnetStreamMaxMsgs    int64
	xnetStreamMaxBytes   int64
	xnetLimitsMu         sync.Mutex
	xnetLimitsConfigured bool
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
		conn:               nc,
		js:                 jetstream,
		xnetStreamMaxMsgs:  cfg.Xnet.MigrationStreamMaxMsgs,
		xnetStreamMaxBytes: cfg.Xnet.MigrationStreamMaxBytes,
	}, nil
}

func (n *Nats) Publish(topic string, data []byte) error {
	if topic == LfsMigrationSubject {
		if err := n.ensureXnetMigrationStreamLimits(); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := n.js.Publish(ctx, topic, data)
	if err != nil {
		return fmt.Errorf("[nats] failed to publish message to subject %s error: %w", topic, err)
	}
	return nil
}

func (n *Nats) ensureXnetMigrationStreamLimits() error {
	if n.xnetStreamMaxMsgs <= 0 || n.xnetStreamMaxBytes <= 0 {
		return nil
	}
	n.xnetLimitsMu.Lock()
	defer n.xnetLimitsMu.Unlock()
	if n.xnetLimitsConfigured {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stream, err := n.js.Stream(ctx, LfsXnetMigrateGroup.StreamName)
	if err != nil {
		return fmt.Errorf("[nats] get existing xnet migration stream: %w", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return fmt.Errorf("[nats] get xnet migration stream info: %w", err)
	}
	config, err := applyXnetMigrationStreamLimits(info.Config, n.xnetStreamMaxMsgs, n.xnetStreamMaxBytes)
	if err != nil {
		return err
	}
	if _, err := n.js.UpdateStream(ctx, config); err != nil {
		return fmt.Errorf("[nats] update xnet migration stream limits: %w", err)
	}
	n.xnetLimitsConfigured = true
	return nil
}

func applyXnetMigrationStreamLimits(config jetstream.StreamConfig, maxMsgs, maxBytes int64) (jetstream.StreamConfig, error) {
	if err := validateXnetMigrationStreamConfig(config); err != nil {
		return jetstream.StreamConfig{}, err
	}
	config.MaxMsgs = maxMsgs
	config.MaxBytes = maxBytes
	config.Discard = jetstream.DiscardNew
	return config, nil
}

func validateXnetMigrationStreamConfig(config jetstream.StreamConfig) error {
	if len(config.Subjects) != 1 || config.Subjects[0] != LfsMigrationSubject {
		return fmt.Errorf(
			"[nats] stream %s must exclusively own subject %s", config.Name, LfsMigrationSubject,
		)
	}
	if config.Retention != jetstream.WorkQueuePolicy {
		return fmt.Errorf("[nats] stream %s must use work queue retention", config.Name)
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
				Acker: msg,
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
			// NATS JetStream's unlimited sentinel for byte limit is -1.
			// Values <= 0 from SubscribeParams are mapped to -1.
			// See https://nats-io.github.io/nats.net/api/NATS.Client.JetStream.Models.StreamConfig.html
			MaxBytes: maxBytesOrDefault(params.MaxBytes),
			// NATS JetStream treats MaxAge 0 as unlimited (no age-based expiry).
			// See https://nats-io.github.io/nats.net/api/NATS.Client.JetStream.Models.StreamConfig.MaxAge.html
			MaxAge: params.MaxAge,
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
			AckWait:        ackWaitOrDefault(params.AckWait),
			MaxDeliver:     maxDeliverOrDefault(params.MaxDeliver),
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

	if streamName == LfsXnetMigrateGroup.StreamName {
		err = purgeXnetMigrationStream(ctx, stream)
	} else {
		err = stream.Purge(ctx)
	}
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
	if streamName == LfsXnetMigrateGroup.StreamName {
		return deleteXnetMessagesByFilter(ctx, stream, filter)
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

type xnetStreamMutation interface {
	Info(context.Context, ...jetstream.StreamInfoOpt) (*jetstream.StreamInfo, error)
	GetMsg(context.Context, uint64, ...jetstream.GetMsgOpt) (*jetstream.RawStreamMsg, error)
	DeleteMsg(context.Context, uint64) error
	Purge(context.Context, ...jetstream.StreamPurgeOpt) error
}

func validatedXnetStreamInfo(ctx context.Context, stream xnetStreamMutation) (*jetstream.StreamInfo, error) {
	info, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("[nats] failed to get xnet migration stream info: %w", err)
	}
	if err := validateXnetMigrationStreamConfig(info.Config); err != nil {
		return nil, err
	}
	return info, nil
}

func purgeXnetMigrationStream(ctx context.Context, stream xnetStreamMutation) error {
	if _, err := validatedXnetStreamInfo(ctx, stream); err != nil {
		return err
	}
	return stream.Purge(ctx, jetstream.WithPurgeSubject(LfsMigrationSubject))
}

func deleteXnetMessagesByFilter(ctx context.Context, stream xnetStreamMutation, filter func(data []byte) bool) error {
	info, err := validatedXnetStreamInfo(ctx, stream)
	if err != nil {
		return err
	}
	if info.State.Msgs == 0 {
		return nil
	}

	deletedCount := 0
	for sequence := info.State.FirstSeq; sequence <= info.State.LastSeq; {
		msg, err := stream.GetMsg(ctx, sequence, jetstream.WithGetMsgSubject(LfsMigrationSubject))
		if errors.Is(err, jetstream.ErrMsgNotFound) {
			break
		}
		if err != nil {
			return fmt.Errorf("[nats] failed to read xnet migration message at or after sequence %d: %w", sequence, err)
		}
		if msg.Sequence > info.State.LastSeq {
			break
		}
		if filter(msg.Data) {
			if err := stream.DeleteMsg(ctx, msg.Sequence); err != nil {
				if !errors.Is(err, jetstream.ErrMsgNotFound) {
					return fmt.Errorf("[nats] failed to delete xnet migration message at sequence %d: %w", msg.Sequence, err)
				}
			} else {
				deletedCount++
			}
		}
		if msg.Sequence == ^uint64(0) {
			break
		}
		sequence = msg.Sequence + 1
	}

	slog.Info("[nats] xnet migration messages deleted by filter", slog.Int("deleted_count", deletedCount))
	return nil
}

func maxBytesOrDefault(v int64) int64 {
	if v <= 0 {
		return -1
	}
	return v
}

func ackWaitOrDefault(d time.Duration) time.Duration {
	if d <= 0 {
		return 30 * time.Second
	}
	return d
}

func maxDeliverOrDefault(v int) int {
	if v <= 0 {
		return -1
	}
	return v
}
