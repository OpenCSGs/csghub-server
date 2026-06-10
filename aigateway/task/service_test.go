package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	taskprocessor "opencsg.com/csghub-server/aigateway/task/processor"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestAsyncGenerationServiceListPendingGenerations(t *testing.T) {
	store := &fakeAIGenerationMeteringStore{
		generations: []database.AIGeneration{
			{
				ID:                 10,
				ResourceType:       database.AIGenerationResourceTypeVideo,
				ResourceID:         "resource-id",
				ProviderResourceID: "provider-resource-id",
				Status:             string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
			},
		},
	}
	service := NewAsyncGenerationServiceWithDeps(AsyncGenerationServiceDeps{
		Store:           &fakeAIGenerationStore{},
		MeteringStore:   store,
		RefreshInterval: 45 * time.Second,
		BatchSize:       7,
		Processors:      []taskprocessor.ResourceProcessor{&fakeResourceProcessor{resourceType: database.AIGenerationResourceTypeVideo}},
	})

	targets, err := service.ListPendingGenerations(context.Background())

	require.NoError(t, err)
	require.Len(t, targets, 1)
	require.Equal(t, int64(10), targets[0].ID)
	require.Equal(t, "resource-id", targets[0].ResourceID)
	require.Equal(t, 7, store.limit)
	require.WithinDuration(t, time.Now().Add(-45*time.Second), store.staleBefore, time.Second)
}

func TestAsyncGenerationServiceInspectAndMeterRefreshesGeneration(t *testing.T) {
	store := &fakeAIGenerationStore{}
	service := NewAsyncGenerationServiceWithDeps(AsyncGenerationServiceDeps{
		Store:         store,
		MeteringStore: &fakeAIGenerationMeteringStore{},
		Processors: []taskprocessor.ResourceProcessor{
			&fakeResourceProcessor{
				resourceType: database.AIGenerationResourceTypeVideo,
				status: &taskprocessor.GenerationStatus{
					Status:           string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
					Progress:         "0.5",
					ProviderMetadata: map[string]any{"provider_status": "processing"},
				},
			},
		},
	})

	err := service.InspectAndMeter(context.Background(), commontypes.AIGatewayAsyncGenerationTarget{
		ID:                 11,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "resource-id",
		ProviderResourceID: "provider-resource-id",
		ProviderMetadata:   map[string]any{"task_id": "provider-resource-id"},
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusQueued),
		CreatedAt:          time.Now(),
	})

	require.NoError(t, err)
	require.Len(t, store.updates, 1)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusQueued), store.updates[0].fromStatus)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusInProgress), store.updates[0].input.Status)
	require.Equal(t, "0.5", store.updates[0].input.Progress)
	require.Equal(t, "provider-resource-id", store.updates[0].input.ProviderMetadata["task_id"])
	require.Equal(t, "processing", store.updates[0].input.ProviderMetadata["provider_status"])
}

func TestAsyncGenerationServiceInspectAndMeterPublishesCompletedEvent(t *testing.T) {
	eventUUID := uuid.New()
	queue := &fakeMessageQueue{}
	meteringMetadata := &commontypes.MeteringEvent{
		Uuid:         uuid.New(),
		UserUUID:     "user-uuid",
		Value:        3,
		ValueType:    commontypes.CountNumberType,
		Scene:        1,
		OpUID:        "op-uid",
		ResourceID:   "resource-id",
		ResourceName: "model-name",
		CustomerID:   "customer-id",
		Extra:        `{"prompt":"make a video"}`,
	}
	store := &fakeAIGenerationStore{
		publishGeneration: database.AIGeneration{
			ID:               12,
			ResourceType:     database.AIGenerationResourceTypeVideo,
			ResourceID:       "resource-id",
			Status:           string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
			EventUUID:        eventUUID,
			MeteringMetadata: meteringMetadata,
		},
	}
	service := NewAsyncGenerationServiceWithDeps(AsyncGenerationServiceDeps{
		Store:         store,
		MeteringStore: &fakeAIGenerationMeteringStore{},
		EventPublisher: &event.EventPublisher{
			MQ: queue,
		},
		Processors: []taskprocessor.ResourceProcessor{&fakeResourceProcessor{resourceType: database.AIGenerationResourceTypeVideo}},
	})

	err := service.InspectAndMeter(context.Background(), commontypes.AIGatewayAsyncGenerationTarget{
		ID:               12,
		ResourceType:     database.AIGenerationResourceTypeVideo,
		ResourceID:       "resource-id",
		Status:           string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:        eventUUID,
		MeteringMetadata: meteringMetadata,
		CreatedAt:        time.Now(),
	})

	require.NoError(t, err)
	require.Len(t, queue.messages, 1)
	require.Len(t, store.published, 1)

	var meteringEvent commontypes.MeteringEvent
	require.NoError(t, json.Unmarshal(queue.messages[0], &meteringEvent))
	require.Equal(t, eventUUID, meteringEvent.Uuid)
	require.Equal(t, "user-uuid", meteringEvent.UserUUID)
	require.Equal(t, int64(3), meteringEvent.Value)
	require.Equal(t, commontypes.CountNumberType, meteringEvent.ValueType)
	require.JSONEq(t, `{"prompt":"make a video"}`, meteringEvent.Extra)
}

type fakeAIGenerationStore struct {
	updates                []fakeAIGenerationUpdate
	published              []database.AIGeneration
	publishGeneration      database.AIGeneration
	updateWithStatusReturn bool
}

type fakeAIGenerationUpdate struct {
	input      database.AIGeneration
	fromStatus string
}

func (s *fakeAIGenerationStore) Create(ctx context.Context, input database.AIGeneration) (*database.AIGeneration, error) {
	return &input, nil
}

func (s *fakeAIGenerationStore) FindByResourceID(ctx context.Context, resourceType, resourceID string) (*database.AIGeneration, error) {
	return nil, nil
}

func (s *fakeAIGenerationStore) Update(ctx context.Context, input database.AIGeneration) (*database.AIGeneration, error) {
	return &input, nil
}

func (s *fakeAIGenerationStore) UpdateWithStatus(ctx context.Context, input database.AIGeneration, fromStatus string) (bool, error) {
	s.updates = append(s.updates, fakeAIGenerationUpdate{input: input, fromStatus: fromStatus})
	return s.updateWithStatusReturn, nil
}

func (s *fakeAIGenerationStore) UpdateProviderMetadata(ctx context.Context, id int64, providerMetadata map[string]any) error {
	return nil
}

func (s *fakeAIGenerationStore) PublishMeteringEventInTx(ctx context.Context, id int64, publishFn func(database.AIGeneration) error) error {
	input := s.publishGeneration
	if input.ID == 0 {
		input = database.AIGeneration{
			ID:           id,
			ResourceType: database.AIGenerationResourceTypeVideo,
			ResourceID:   "resource-id",
			Status:       string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
			EventUUID:    uuid.New(),
		}
	}
	if err := publishFn(input); err != nil {
		return err
	}
	s.published = append(s.published, input)
	return nil
}

type fakeAIGenerationMeteringStore struct {
	generations []database.AIGeneration
	staleBefore time.Time
	limit       int
}

func (s *fakeAIGenerationMeteringStore) ListMeteringCandidates(ctx context.Context, staleBefore time.Time, limit int) ([]database.AIGeneration, error) {
	s.staleBefore = staleBefore
	s.limit = limit
	return s.generations, nil
}

type fakeResourceProcessor struct {
	resourceType string
	status       *taskprocessor.GenerationStatus
	called       bool
}

func (p *fakeResourceProcessor) ResourceType() string {
	return p.resourceType
}

func (p *fakeResourceProcessor) Refresh(ctx context.Context, ref taskprocessor.GenerationRef) (*taskprocessor.GenerationStatus, error) {
	p.called = true
	return p.status, nil
}

type fakeMessageQueue struct {
	messages [][]byte
}

func (q *fakeMessageQueue) Publish(topic string, data []byte) error {
	q.messages = append(q.messages, data)
	return nil
}

func (q *fakeMessageQueue) Subscribe(params mq.SubscribeParams) error {
	return nil
}

func (q *fakeMessageQueue) PurgeStream(streamName string) error {
	return nil
}

func (q *fakeMessageQueue) DeleteMessagesByFilter(streamName string, filter func(data []byte) bool) error {
	return nil
}

func TestAsyncGenerationServiceInspectAndMeterTimesOutStaleInProgress(t *testing.T) {
	store := &fakeAIGenerationStore{}
	processor := &fakeResourceProcessor{resourceType: database.AIGenerationResourceTypeVideo}
	service := NewAsyncGenerationServiceWithDeps(AsyncGenerationServiceDeps{
		Store:         store,
		MeteringStore: &fakeAIGenerationMeteringStore{},
		MaxAge:        time.Hour,
		Processors:    []taskprocessor.ResourceProcessor{processor},
	})

	now := time.Now()
	err := service.InspectAndMeter(context.Background(), commontypes.AIGatewayAsyncGenerationTarget{
		ID:           101,
		ResourceType: database.AIGenerationResourceTypeVideo,
		ResourceID:   "resource-id",
		Status:       string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
		CreatedAt:    now.Add(-2 * time.Hour),
	})

	require.NoError(t, err)
	require.False(t, processor.called, "processor.Refresh must not be called for timed-out generations")
	require.Len(t, store.updates, 1)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusInProgress), store.updates[0].fromStatus)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusFailed), store.updates[0].input.Status)
	require.Contains(t, store.updates[0].input.FailReason, "max age")
	require.Contains(t, store.updates[0].input.FailReason, "1h0m0s")
	require.NotNil(t, store.updates[0].input.FinishedAt)
	require.WithinDuration(t, now, *store.updates[0].input.FinishedAt, time.Second)
}

func TestAsyncGenerationServiceInspectAndMeterTimesOutStaleQueued(t *testing.T) {
	store := &fakeAIGenerationStore{}
	processor := &fakeResourceProcessor{resourceType: database.AIGenerationResourceTypeVideo}
	service := NewAsyncGenerationServiceWithDeps(AsyncGenerationServiceDeps{
		Store:         store,
		MeteringStore: &fakeAIGenerationMeteringStore{},
		MaxAge:        time.Hour,
		Processors:    []taskprocessor.ResourceProcessor{processor},
	})

	now := time.Now()
	err := service.InspectAndMeter(context.Background(), commontypes.AIGatewayAsyncGenerationTarget{
		ID:           102,
		ResourceType: database.AIGenerationResourceTypeVideo,
		ResourceID:   "resource-id",
		Status:       string(commontypes.AIGatewayAsyncGenerationStatusQueued),
		CreatedAt:    now.Add(-2 * time.Hour),
	})

	require.NoError(t, err)
	require.False(t, processor.called)
	require.Len(t, store.updates, 1)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusQueued), store.updates[0].fromStatus)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusFailed), store.updates[0].input.Status)
	require.Contains(t, store.updates[0].input.FailReason, "max age")
}

func TestAsyncGenerationServiceInspectAndMeterTimeoutLosesCASRace(t *testing.T) {
	store := &fakeAIGenerationStore{updateWithStatusReturn: false}
	processor := &fakeResourceProcessor{resourceType: database.AIGenerationResourceTypeVideo}
	service := NewAsyncGenerationServiceWithDeps(AsyncGenerationServiceDeps{
		Store:         store,
		MeteringStore: &fakeAIGenerationMeteringStore{},
		MaxAge:        time.Hour,
		Processors:    []taskprocessor.ResourceProcessor{processor},
	})

	err := service.InspectAndMeter(context.Background(), commontypes.AIGatewayAsyncGenerationTarget{
		ID:           103,
		ResourceType: database.AIGenerationResourceTypeVideo,
		ResourceID:   "resource-id",
		Status:       string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
		CreatedAt:    time.Now().Add(-2 * time.Hour),
	})

	require.NoError(t, err)
	require.Len(t, store.updates, 1)
}

func TestAsyncGenerationServiceInspectAndMeterDoesNotTimeoutWhenMaxAgeZero(t *testing.T) {
	store := &fakeAIGenerationStore{}
	processor := &fakeResourceProcessor{resourceType: database.AIGenerationResourceTypeVideo}
	service := NewAsyncGenerationServiceWithDeps(AsyncGenerationServiceDeps{
		Store:         store,
		MeteringStore: &fakeAIGenerationMeteringStore{},
		MaxAge:        0,
		Processors:    []taskprocessor.ResourceProcessor{processor},
	})

	err := service.InspectAndMeter(context.Background(), commontypes.AIGatewayAsyncGenerationTarget{
		ID:           104,
		ResourceType: database.AIGenerationResourceTypeVideo,
		ResourceID:   "resource-id",
		Status:       string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
		CreatedAt:    time.Now().Add(-24 * time.Hour),
	})

	require.NoError(t, err)
	require.True(t, processor.called, "processor.Refresh must run when the timeout is disabled")
	require.Empty(t, store.updates, "no timeout-driven status change when MaxAge is zero")
}
