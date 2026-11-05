package cache

import (
	"context"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	riskRuleCacheTTL    = time.Minute
	riskRuleErrorTTL    = 5 * time.Second
	riskRuleLoadTimeout = 5 * time.Second
)

// RiskRuleSnapshot is an immutable set of rules published by RiskRuleCache.
type RiskRuleSnapshot struct {
	rules     map[string]struct{}
	err       error
	expiresAt time.Time
}

// RiskRuleCache keeps rule snapshots in memory and merges concurrent refreshes.
// Its zero value is ready for use.
type RiskRuleCache struct {
	current atomic.Pointer[RiskRuleSnapshot]
	refresh singleflight.Group
	now     func() time.Time // Optional clock for deterministic cache tests.
}

func (c *RiskRuleCache) timeNow() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// Get returns a fresh snapshot or an error; it never serves expired rules after a failed refresh.
func (c *RiskRuleCache) Get(ctx context.Context, load func(context.Context) (map[string]struct{}, error)) (*RiskRuleSnapshot, error) {
	if snapshot := c.current.Load(); snapshot != nil && c.timeNow().Before(snapshot.expiresAt) {
		return snapshot, snapshot.err
	}
	result := c.refresh.DoChan("rules", func() (any, error) {
		if snapshot := c.current.Load(); snapshot != nil && c.timeNow().Before(snapshot.expiresAt) {
			return snapshot, nil
		}
		// One cancelled caller must not cancel a refresh shared by other requests.
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), riskRuleLoadTimeout)
		defer cancel()
		rules, err := load(loadCtx)
		ttl := riskRuleCacheTTL
		if err != nil {
			ttl = riskRuleErrorTTL
			rules = nil
		}
		snapshot := &RiskRuleSnapshot{rules: rules, err: err, expiresAt: c.timeNow().Add(ttl)}
		c.current.Store(snapshot)
		return snapshot, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case loaded := <-result:
		snapshot := loaded.Val.(*RiskRuleSnapshot)
		return snapshot, snapshot.err
	}
}

// Contains reports whether a rule exists without exposing the mutable map.
func (s *RiskRuleSnapshot) Contains(rule string) bool {
	_, found := s.rules[rule]
	return found
}
