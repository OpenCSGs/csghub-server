package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

// meteringMQ is a local mq.MessageQueue fake that can be told to fail publishes.
// It exists separately from the fakeMessageQueue in service_test.go so this
// test file is self-contained.
type meteringMQ struct {
	messages [][]byte
	err      error
}

func (q *meteringMQ) Publish(topic string, data []byte) error {
	if q.err != nil {
		return q.err
	}
	q.messages = append(q.messages, data)
	return nil
}

func (q *meteringMQ) Subscribe(_ mq.SubscribeParams) error       { return nil }
func (q *meteringMQ) PurgeStream(_ string) error                { return nil }
func (q *meteringMQ) DeleteMessagesByFilter(_ string, _ func([]byte) bool) error {
	return nil
}

func newMeteringService(q *meteringMQ) *asyncGenerationService {
	return &asyncGenerationService{
		eventPub: &event.EventPublisher{MQ: q},
	}
}

func TestPublishMeteringEventReturnsErrorWhenPublisherNotConfigured(t *testing.T) {
	s := &asyncGenerationService{eventPub: nil}
	eventUUID := uuid.New()

	err := s.publishMeteringEvent(context.Background(), &database.AIGeneration{
		ID:           1,
		ResourceType: "video",
		ResourceID:   "r1",
		Status:       string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:    eventUUID,
		MeteringMetadata: &commontypes.MeteringEvent{
			Uuid:       eventUUID,
			ResourceID: "r1",
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "publisher is not configured")
}

func TestPublishMeteringEventReturnsErrorWhenMetadataMissing(t *testing.T) {
	q := &meteringMQ{}
	s := newMeteringService(q)

	err := s.publishMeteringEvent(context.Background(), &database.AIGeneration{
		ID:               1,
		ResourceType:     "video",
		ResourceID:       "r1",
		Status:           string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:        uuid.New(),
		MeteringMetadata: nil,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "missing async generation metering event metadata")
	require.Empty(t, q.messages, "nothing must be published when metadata is missing")
}

func TestPublishMeteringEventPublishesWithOverriddenUuid(t *testing.T) {
	q := &meteringMQ{}
	s := newMeteringService(q)

	storedUUID := uuid.New()
	eventUUID := uuid.New() // different — must take precedence over the metadata's Uuid

	err := s.publishMeteringEvent(context.Background(), &database.AIGeneration{
		ID:           1,
		ResourceType: "video",
		ResourceID:   "r1",
		Status:       string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:    eventUUID,
		MeteringMetadata: &commontypes.MeteringEvent{
			Uuid:       storedUUID,
			UserUUID:   "user-1",
			Value:      7,
			ValueType:  commontypes.CountNumberType,
			Scene:      1,
			ResourceID: "r1",
			Extra:      `{"prompt":"a cat"}`,
		},
	})

	require.NoError(t, err)
	require.Len(t, q.messages, 1, "exactly one message must be published")
	var published commontypes.MeteringEvent
	require.NoError(t, json.Unmarshal(q.messages[0], &published))
	require.Equal(t, eventUUID, published.Uuid,
		"generation.EventUUID must override the Uuid field in MeteringMetadata")
	require.NotEqual(t, storedUUID, eventUUID, "sanity: eventUUID must differ from storedUUID for this test to be meaningful")
	require.Equal(t, "user-1", published.UserUUID)
	require.Equal(t, int64(7), published.Value)
	require.Equal(t, commontypes.CountNumberType, published.ValueType)
	require.Equal(t, `{"prompt":"a cat"}`, published.Extra)
}

func TestPublishMeteringEventWrapsPublishError(t *testing.T) {
	q := &meteringMQ{err: errors.New("nats offline")}
	s := newMeteringService(q)

	err := s.publishMeteringEvent(context.Background(), &database.AIGeneration{
		ID:           1,
		ResourceType: "video",
		ResourceID:   "r1",
		Status:       string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:    uuid.New(),
		MeteringMetadata: &commontypes.MeteringEvent{
			ResourceID: "r1",
		},
	})

	require.Error(t, err)
	require.ErrorIs(t, err, q.err)
	require.Contains(t, err.Error(), "publish async generation metering event")
}

func TestPublishMeteringEventPropagatesMeteringFields(t *testing.T) {
	// Verifies that all scalar fields of MeteringEvent survive the deref +
	// Uuid override without any field being dropped on the way to the queue.
	q := &meteringMQ{}
	s := newMeteringService(q)

	in := &commontypes.MeteringEvent{
		Uuid:         uuid.New(),
		UserUUID:     "user-99",
		Value:        42,
		ValueType:    commontypes.TimeDurationMinType,
		Scene:        7,
		OpUID:        "op-1",
		ResourceID:   "r-1",
		ResourceName: "res-name",
		CustomerID:   "cust-1",
		Extra:        `{"key":"value"}`,
	}
	eventUUID := uuid.New()

	err := s.publishMeteringEvent(context.Background(), &database.AIGeneration{
		EventUUID:        eventUUID,
		MeteringMetadata: in,
	})

	require.NoError(t, err)
	require.Len(t, q.messages, 1)
	var out commontypes.MeteringEvent
	require.NoError(t, json.Unmarshal(q.messages[0], &out))
	require.Equal(t, eventUUID, out.Uuid)
	require.Equal(t, in.UserUUID, out.UserUUID)
	require.Equal(t, in.Value, out.Value)
	require.Equal(t, in.ValueType, out.ValueType)
	require.Equal(t, in.Scene, out.Scene)
	require.Equal(t, in.OpUID, out.OpUID)
	require.Equal(t, in.ResourceID, out.ResourceID)
	require.Equal(t, in.ResourceName, out.ResourceName)
	require.Equal(t, in.CustomerID, out.CustomerID)
	require.Equal(t, in.Extra, out.Extra)
}
