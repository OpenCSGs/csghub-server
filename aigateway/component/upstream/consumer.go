package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	bldmq "opencsg.com/csghub-server/builder/mq"
	commontypes "opencsg.com/csghub-server/common/types"
)

// UpstreamSyncConsumer subscribes to deploy lifecycle MQ events and
// dispatches them to the AIGatewayUpstreamSyncComponent.
type UpstreamSyncConsumer struct {
	mq       bldmq.MessageQueue
	syncComp AIGatewayUpstreamSyncComponent
	ctx      context.Context
	cancel   context.CancelFunc

	// lastEventTime tracks the highest EventTime seen per deployID so that
	// stale events (e.g. a delayed running event arriving after a stop or
	// delete) can be discarded. This is an in-process guard; on restart the
	// map is empty and the periodic reconcile corrects any drift.
	lastEventTime   map[int64]int64
	lastEventTimeMu sync.Mutex
}

func NewUpstreamSyncConsumer(mq bldmq.MessageQueue, syncComp AIGatewayUpstreamSyncComponent) *UpstreamSyncConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &UpstreamSyncConsumer{
		mq:            mq,
		syncComp:      syncComp,
		ctx:           ctx,
		cancel:        cancel,
		lastEventTime: make(map[int64]int64),
	}
}

// Start subscribes to the three deploy upstream sync subjects.
// It blocks until the context is cancelled or all subscriptions fail.
func (c *UpstreamSyncConsumer) Start() error {
	topics := []string{
		bldmq.DeployUpstreamSyncRunningSubject,
		bldmq.DeployUpstreamSyncStopSubject,
		bldmq.DeployUpstreamSyncDeleteSubject,
	}

	// AutoACK is false: the callback explicitly ACKs on success and returns an
	// error on failure so the MQ layer redelivers the message (see handleMessage).
	// NOTE on backends: the NATS implementation provides a working Acker and
	// honors explicit Ack(); the Kafka implementation does NOT provide an Acker,
	// so the explicit Ack() calls are no-ops there and Kafka relies on its own
	// consumer-group offset semantics for delivery. This consumer is intended to
	// run against the NATS backend; using Kafka would require an Acker shim.
	err := c.mq.Subscribe(bldmq.SubscribeParams{
		Group:                  bldmq.DeployUpstreamSyncGroup,
		Topics:                 topics,
		AutoACK:                false,
		IsRedeliverForCBFailed: true,
		MaxDeliver:             5,
		AckWait:                30 * time.Second,
		Callback:               c.handleMessage,
	})
	if err != nil {
		return fmt.Errorf("upstream sync consumer: subscribe failed: %w", err)
	}

	slog.InfoContext(c.ctx, "upstream sync consumer started",
		slog.Any("topics", topics),
		slog.String("group", bldmq.DeployUpstreamSyncGroup.ConsumerName),
	)
	return nil
}

// Stop cancels the consumer context.
func (c *UpstreamSyncConsumer) Stop() {
	c.cancel()
}

func (c *UpstreamSyncConsumer) handleMessage(raw []byte, meta bldmq.MessageMeta) error {
	var event commontypes.DeployUpstreamSyncEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		// Poison pill: ACK and drop invalid JSON to avoid infinite redelivery.
		slog.ErrorContext(c.ctx, "upstream sync consumer: failed to unmarshal event, discarding",
			slog.Any("error", err),
			slog.String("topic", meta.Topic),
		)
		if meta.Acker != nil {
			_ = meta.Acker.Ack()
		}
		return nil
	}

	slog.DebugContext(c.ctx, "upstream sync consumer: received event",
		slog.String("topic", meta.Topic),
		slog.Int64("deploy_id", event.DeployID),
		slog.Int64("event_time", event.EventTime),
	)

	// Reject stale events: if we have already processed a newer event for
	// this deploy, discard this one. This prevents a delayed running event
	// from resurrecting an upstream that was already stopped or deleted.
	if c.isStaleEvent(event.DeployID, event.EventTime) {
		slog.InfoContext(c.ctx, "upstream sync consumer: discarding stale event",
			slog.String("topic", meta.Topic),
			slog.Int64("deploy_id", event.DeployID),
			slog.Int64("event_time", event.EventTime),
		)
		if meta.Acker != nil {
			_ = meta.Acker.Ack()
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	var err error
	switch meta.Topic {
	case bldmq.DeployUpstreamSyncRunningSubject:
		if event.Deploy == nil {
			slog.ErrorContext(ctx, "upstream sync consumer: running event missing deploy info",
				slog.Int64("deploy_id", event.DeployID))
			return fmt.Errorf("running event for deploy %d missing deploy info", event.DeployID)
		}
		err = c.syncComp.SyncRunningDeploy(ctx, event.Deploy)
	case bldmq.DeployUpstreamSyncStopSubject:
		err = c.syncComp.DisableDeployTarget(ctx, event.DeployID)
	case bldmq.DeployUpstreamSyncDeleteSubject:
		err = c.syncComp.DeleteDeployTarget(ctx, event.DeployID)
	default:
		slog.WarnContext(ctx, "upstream sync consumer: unknown topic, skipping",
			slog.String("topic", meta.Topic),
		)
		if meta.Acker != nil {
			_ = meta.Acker.Ack()
		}
		return nil
	}

	if err != nil {
		// Return error so the MQ layer redelivers the message. With AutoACK=false
		// the MQ layer does NOT auto-NAK: on NATS the message is left unacked and
		// is redelivered after the AckWait (30s) timeout rather than immediately;
		// on Kafka, where no Acker is provided, redelivery happens on the next poll.
		// MaxDeliver bounds the retry count, after which the message is dead-lettered.
		slog.ErrorContext(ctx, "upstream sync consumer: failed to process event, returning error for redelivery",
			slog.String("topic", meta.Topic),
			slog.Int64("deploy_id", event.DeployID),
			slog.Any("error", err),
		)
		return err
	}

	// Record the event time only on success so that a failed event can be
	// retried and processed when redelivered.
	c.recordEventTime(event.DeployID, event.EventTime)

	// Explicit ACK on success.
	if meta.Acker != nil {
		_ = meta.Acker.Ack()
	}
	return nil
}

// isStaleEvent returns true if the consumer has already processed an event
// with a higher or equal EventTime for the same deployID. EventTime is a
// monotonically increasing timestamp set by the publisher.
func (c *UpstreamSyncConsumer) isStaleEvent(deployID, eventTime int64) bool {
	// Events without a timestamp (EventTime == 0) are always processed to
	// maintain backward compatibility with older publishers.
	if eventTime == 0 {
		return false
	}
	c.lastEventTimeMu.Lock()
	defer c.lastEventTimeMu.Unlock()
	last, ok := c.lastEventTime[deployID]
	if !ok {
		return false
	}
	return eventTime <= last
}

// recordEventTime stores the EventTime for a deployID after the event has
// been successfully processed.
func (c *UpstreamSyncConsumer) recordEventTime(deployID, eventTime int64) {
	if eventTime == 0 {
		return
	}
	c.lastEventTimeMu.Lock()
	defer c.lastEventTimeMu.Unlock()
	if eventTime > c.lastEventTime[deployID] {
		c.lastEventTime[deployID] = eventTime
	}
}
