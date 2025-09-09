package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
	"opencsg.com/csghub-server/common/config"
)

var ErrLockAcquire = errors.New("distributed lock acquire fail")
var TakenErr *redsync.ErrTaken

type DistributedLocker struct {
	RedSync                  *redsync.Redsync
	MutexServerAcctMetering  *redsync.Mutex
	MutexRunnerAcctMetering  *redsync.Mutex
	MutexDeployTaskScheudler *redsync.Mutex
}

var (
	mutexServerAcctMeteringLocker  = "mutex_server_acct_metering"
	mutexRunnerAcctMeteringLocker  = "mutex_runner_acct_metering"
	mutexDeployTaskSchedulerLocker = "mutex_deploy_task_scheduler"
	lockContextTimeout             = 1 * time.Second
	lockTimeroutFactor             = 1.0
	lockExpiredTimeout             = 61 * time.Second
)

func newMutexWithOptions(redSync *redsync.Redsync, name, value string) *redsync.Mutex {
	return redSync.NewMutex(
		name,
		redsync.WithExpiry(lockExpiredTimeout),
		redsync.WithTimeoutFactor(lockTimeroutFactor),
		redsync.WithValue(value),
	)
}

func InitDistributedLocker(cfg *config.Config) *DistributedLocker {
	redisClient := goredislib.NewClient(&goredislib.Options{
		Addr:         cfg.Redis.Endpoint,
		MinIdleConns: cfg.Redis.MinIdleConnections,
		Username:     cfg.Redis.User,
		Password:     cfg.Redis.Password,
		OnConnect: func(ctx context.Context, cn *goredislib.Conn) error {
			slog.Debug(fmt.Sprintf("Connected to Redis %s", cfg.Redis.Endpoint))
			return nil
		},
	})
	redSync := redsync.New(goredis.NewPool(redisClient))
	return &DistributedLocker{
		RedSync:                  redSync,
		MutexServerAcctMetering:  newMutexWithOptions(redSync, mutexServerAcctMeteringLocker, cfg.UniqueServiceName),
		MutexRunnerAcctMetering:  newMutexWithOptions(redSync, mutexRunnerAcctMeteringLocker, cfg.UniqueServiceName),
		MutexDeployTaskScheudler: newMutexWithOptions(redSync, mutexDeployTaskSchedulerLocker, cfg.UniqueServiceName),
	}
}

func (r *DistributedLocker) getLock(mutex *redsync.Mutex) error {
	ctx, cancel := context.WithTimeout(context.Background(), lockContextTimeout)
	defer cancel()
	err := mutex.TryLockContext(ctx)
	if err != nil {
		if errors.As(err, &TakenErr) {
			//case: lock already taken, locked nodes: [0]
			// *v4.ErrTaken {Nodes: []int len: 1, cap: 1, [0]}
			slog.Debug("get distributed lock error", slog.Any("error", err))
			return ErrLockAcquire
		}
		return fmt.Errorf("failed to acquire lock %s with %s in redis error: %w", mutex.Name(), mutex.Value(), err)
	}
	return nil
}

func (r *DistributedLocker) releaseLock(mutex *redsync.Mutex) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), lockContextTimeout)
	defer cancel()
	ok, err := mutex.UnlockContext(ctx)
	if err != nil {
		return ok, fmt.Errorf("unlock %s with %s from redis error: %w", mutex.Name(), mutex.Value(), err)
	}
	return ok, nil
}

func (r *DistributedLocker) GetServerAcctMeteringLock() error {
	return r.getLock(r.MutexServerAcctMetering)
}

func (r *DistributedLocker) ReleaseServerAcctMeteringLock() (bool, error) {
	return r.releaseLock(r.MutexServerAcctMetering)
}

func (r *DistributedLocker) GetRunnerAcctMeteringLock() error {
	return r.getLock(r.MutexRunnerAcctMetering)
}

func (r *DistributedLocker) ReleaseRunnerAcctMeteringLock() (bool, error) {
	return r.releaseLock(r.MutexRunnerAcctMetering)
}

func (r *DistributedLocker) GetDeployTaskSchedulerLock() error {
	return r.getLock(r.MutexDeployTaskScheudler)
}
func (r *DistributedLocker) ReleaseDeployTaskSchedulerLock() (bool, error) {
	return r.releaseLock(r.MutexDeployTaskScheudler)
}
