package task

import (
	"context"
	"strings"
	"time"

	aigatewaycomp "opencsg.com/csghub-server/aigateway/component"
	taskprocessor "opencsg.com/csghub-server/aigateway/task/processor"
	"opencsg.com/csghub-server/aigateway/task/processor/video"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	defaultStatusRefreshInterval = 60 * time.Second
	defaultMeteringBatchSize     = 100
	defaultAsyncGenerationMaxAge = 24 * time.Hour
)

type AsyncGenerationService interface {
	ListPendingGenerations(ctx context.Context) ([]commontypes.AIGatewayAsyncGenerationTarget, error)
	InspectAndMeter(ctx context.Context, target commontypes.AIGatewayAsyncGenerationTarget) error
}

type AsyncGenerationServiceDeps struct {
	Store           database.AIGenerationStore
	MeteringStore   database.AIGenerationMeteringStore
	EventPublisher  *event.EventPublisher
	RefreshInterval time.Duration
	BatchSize       int
	MaxAge          time.Duration
	Processors      []taskprocessor.ResourceProcessor
}

type asyncGenerationService struct {
	store           database.AIGenerationStore
	meteringStore   database.AIGenerationMeteringStore
	eventPub        *event.EventPublisher
	refreshInterval time.Duration
	batchSize       int
	maxAge          time.Duration
	processors      map[string]taskprocessor.ResourceProcessor
}

var _ AsyncGenerationService = (*asyncGenerationService)(nil)

func NewAsyncGenerationService(cfg *config.Config) (AsyncGenerationService, error) {
	openAIComponent, err := aigatewaycomp.NewOpenAIComponentFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewAsyncGenerationServiceWithDeps(AsyncGenerationServiceDeps{
		Store:           database.NewAIGenerationStore(),
		MeteringStore:   database.NewAIGenerationMeteringStore(),
		EventPublisher:  &event.DefaultEventPublisher,
		RefreshInterval: statusRefreshInterval(cfg),
		BatchSize:       meteringBatchSize(cfg),
		MaxAge:          asyncGenerationMaxAgeFromConfig(cfg),
		Processors: []taskprocessor.ResourceProcessor{
			video.NewProcessor(openAIComponent, nil, nil),
		},
	}), nil
}

func NewAsyncGenerationServiceWithDeps(deps AsyncGenerationServiceDeps) AsyncGenerationService {
	if deps.Store == nil {
		deps.Store = database.NewAIGenerationStore()
	}
	if deps.MeteringStore == nil {
		deps.MeteringStore = database.NewAIGenerationMeteringStore()
	}
	if deps.EventPublisher == nil {
		deps.EventPublisher = &event.DefaultEventPublisher
	}
	if deps.RefreshInterval <= 0 {
		deps.RefreshInterval = defaultStatusRefreshInterval
	}
	if deps.BatchSize <= 0 {
		deps.BatchSize = defaultMeteringBatchSize
	}
	if deps.MaxAge < 0 {
		deps.MaxAge = defaultAsyncGenerationMaxAge
	}

	return &asyncGenerationService{
		store:           deps.Store,
		meteringStore:   deps.MeteringStore,
		eventPub:        deps.EventPublisher,
		refreshInterval: deps.RefreshInterval,
		batchSize:       deps.BatchSize,
		maxAge:          deps.MaxAge,
		processors:      buildProcessorMap(deps.Processors),
	}
}

func statusRefreshInterval(cfg *config.Config) time.Duration {
	if cfg != nil && cfg.AIGateway.AsyncGenerationStatusRefreshInterval > 0 {
		return time.Duration(cfg.AIGateway.AsyncGenerationStatusRefreshInterval) * time.Second
	}
	return defaultStatusRefreshInterval
}

func meteringBatchSize(cfg *config.Config) int {
	if cfg != nil && cfg.AIGateway.AsyncGenerationMeteringBatchSize > 0 {
		return cfg.AIGateway.AsyncGenerationMeteringBatchSize
	}
	return defaultMeteringBatchSize
}

func asyncGenerationMaxAgeFromConfig(cfg *config.Config) time.Duration {
	if cfg != nil && cfg.AIGateway.AsyncGenerationMaxAge > 0 {
		return time.Duration(cfg.AIGateway.AsyncGenerationMaxAge) * time.Second
	}
	return defaultAsyncGenerationMaxAge
}

func isTerminalStatus(status string) bool {
	switch normalizeStatus(status) {
	case string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		string(commontypes.AIGatewayAsyncGenerationStatusFailed),
		string(commontypes.AIGatewayAsyncGenerationStatusCancelled):
		return true
	default:
		return false
	}
}

func isCompletedStatus(status string) bool {
	return normalizeStatus(status) == string(commontypes.AIGatewayAsyncGenerationStatusCompleted)
}

func normalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}
