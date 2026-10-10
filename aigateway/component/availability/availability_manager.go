package availability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

// AvailabilityManager manages health checking and circuit breaking
type AvailabilityManager interface {
	Start(ctx context.Context) error
	Stop() error
	RecordRequestResult(ctx context.Context, upstreamID int64, modelID string, success bool, err error) error
	GetCircuitState(ctx context.Context, upstreamID int64) (*types.ProviderCircuitStatus, error)
}

type availabilityManagerImpl struct {
	healthChecker  HealthChecker
	circuitBreaker CircuitBreaker
	cancel         context.CancelFunc
}

// NewAvailabilityManagerFromConfig creates a new availability manager from config
func NewAvailabilityManagerFromConfig(cfg *config.Config) (AvailabilityManager, error) {
	// Create stores
	healthStore := database.NewAIGatewayUpstreamHealthStateStore()
	circuitStore := database.NewAIGatewayUpstreamCircuitStateStore()
	upstreamStore := database.NewUpstreamStore(cfg)
	llmConfigStore := database.NewLLMConfigStore(cfg)

	// Create Redis client
	var redisClient cache.RedisClient
	var err error
	if cfg.Redis.Endpoint != "" {
		redisClient, err = cache.NewCache(context.Background(), cache.RedisConfig{
			Addr:     cfg.Redis.Endpoint,
			Username: cfg.Redis.User,
			Password: cfg.Redis.Password,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create redis client: %w", err)
		}
	}

	// Create circuit breaker config from Config
	circuitConfig := types.CircuitBreakerConfig{
		Enabled:             cfg.AIGateway.CircuitBreakerEnabled,
		FailureThreshold:    cfg.AIGateway.CircuitBreakerFailureThreshold,
		ErrorRateThreshold:  0.5,
		SlidingWindowSize:   10,
		OpenDuration:        time.Duration(cfg.AIGateway.CircuitBreakerOpenDuration) * time.Second,
		HalfOpenMaxRequests: cfg.AIGateway.CircuitBreakerHalfOpenMax,
	}

	// Create components
	circuitBreaker := NewCircuitBreaker(circuitConfig, circuitStore, redisClient)
	healthChecker := NewHealthChecker(circuitBreaker, cfg, healthStore, upstreamStore, llmConfigStore, redisClient)

	return &availabilityManagerImpl{
		healthChecker:  healthChecker,
		circuitBreaker: circuitBreaker,
	}, nil
}

func (m *availabilityManagerImpl) Start(ctx context.Context) error {
	managerCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	if err := m.healthChecker.Start(managerCtx); err != nil {
		cancel()
		return err
	}
	if err := m.circuitBreaker.Start(managerCtx); err != nil {
		cancel()
		_ = m.healthChecker.Stop()
		return err
	}
	return nil
}

func (m *availabilityManagerImpl) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}
	circuitErr := m.circuitBreaker.Stop()
	healthErr := m.healthChecker.Stop()
	return errors.Join(circuitErr, healthErr)
}

func (m *availabilityManagerImpl) RecordRequestResult(ctx context.Context, upstreamID int64, modelID string, success bool, err error) error {
	if success {
		return m.circuitBreaker.RecordSuccess(ctx, upstreamID)
	}
	return m.circuitBreaker.RecordFailure(ctx, upstreamID, modelID, err)
}

func (m *availabilityManagerImpl) GetCircuitState(ctx context.Context, upstreamID int64) (*types.ProviderCircuitStatus, error) {
	return m.circuitBreaker.GetCircuitState(ctx, upstreamID)
}
