package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const lockKeyPrefix = "lock-"

var (
	ErrResourceAlreadyLocked = errors.New("resource is already locked")
	ErrResourceNotLocked     = errors.New("resource is not locked")
	ErrResourceConflict      = errors.New("failed to unlock a resource which is locked by others")
)

// RunWhileLocked locks the resource, runs user provided callback and finally release the lock.
// If the lock is not acquirable, this method returns ErrResourceAlreadyLocked immediately.
//
// Ref: https://redis.io/docs/manual/patterns/distributed-locks/#correct-implementation-with-a-single-instance
func (c *Cache) RunWhileLocked(ctx context.Context, resourceName string, expiration time.Duration, fn func(ctx context.Context) error) (err error) {
	return c.lockAndRun(ctx, resourceName, expiration, false, fn)
}

// WaitLockToRun wait for the lock to be acquirable and locks the resource, runs user provided callback and finally release the lock.
// If ctx is canceled or timed out during the waiting, this method returns the error of the ctx.
func (c *Cache) WaitLockToRun(ctx context.Context, resourceName string, expiration time.Duration, fn func(ctx context.Context) error) (err error) {
	return c.lockAndRun(ctx, resourceName, expiration, true, fn)
}

func (c *Cache) lockAndRun(ctx context.Context, resourceName string, expiration time.Duration, waitUntilAvailable bool, fn func(ctx context.Context) error) (err error) {
	randomStr, err := c.acquireLock(ctx, resourceName, expiration, waitUntilAvailable)
	if err != nil {
		err = fmt.Errorf("acquiring lock: %w", err)
		return
	}

	defer func() {
		// ctx may be timed out
		releasingCtx, cancelRelease := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelRelease()

		releasingErr := c.releaseLock(releasingCtx, resourceName, randomStr)
		if releasingErr == nil {
			return
		}
		if err != nil {
			return
		}
		err = fmt.Errorf("releasing lock: %w", releasingErr)
	}()

	err = fn(ctx)
	return
}

// acquireLock implements poor man's resource lock based on single-instance Redis.
//
// Ref: https://redis.io/docs/manual/patterns/distributed-locks/#correct-implementation-with-a-single-instance
func (c *Cache) acquireLock(ctx context.Context, resourceName string, expiration time.Duration, waitUntilAvailable bool) (randomStr string, err error) {
	if resourceName == "" {
		err = errors.New("resource name can not be empty")
		return
	}
	key := lockKeyPrefix + resourceName

	randomStr = uuid.New().String()

	ok, err := c.core.SetNX(ctx, key, randomStr, expiration).Result()
	if err != nil {
		err = fmt.Errorf("calling redis SETNX: %w", err)
		return
	}
	if ok {
		return
	}
	if !waitUntilAvailable {
		err = ErrResourceAlreadyLocked
		return
	}

	// try to acquire the lock from time to time, until success or time out
	const (
		initialInterval = 50 * time.Millisecond
		maxInterval     = 1 * time.Second
	)
	var (
		interval = initialInterval
		timer    = time.NewTimer(interval)
	)
	for {
		select {
		case <-ctx.Done():
			// prevent resource leak
			if !timer.Stop() {
				<-timer.C
			}

			err = fmt.Errorf("waiting for lock: %w", ctx.Err())
			return
		case <-timer.C:
			ok, err = c.core.SetNX(ctx, key, randomStr, expiration).Result()
			if err != nil {
				err = fmt.Errorf("calling redis SETNX: %w", err)
				return
			}
			if ok {
				return
			}

			interval *= 2
			if interval > maxInterval {
				interval = maxInterval
			}
			timer.Reset(interval)
		}
	}
}

// ReleaseLock releases lock created by AcquireLock.
//
// Ref: https://redis.io/docs/manual/patterns/distributed-locks/#correct-implementation-with-a-single-instance
func (c *Cache) releaseLock(ctx context.Context, resourceName string, providedRandomStr string) (err error) {
	signal, err := c.releaseLockScript.Run(ctx, c.core, []string{lockKeyPrefix + resourceName}, []interface{}{providedRandomStr}).Int()
	if err != nil {
		err = fmt.Errorf("running redis script: %w", err)
		return
	}

	switch signal {
	case -1:
		err = ErrResourceNotLocked
	case 0:
		err = ErrResourceConflict
	case 1:
		// relax
	default:
		// not reached
		panic(fmt.Errorf("unexpected signal %d on resourceName %s", signal, resourceName))
	}

	return
}
