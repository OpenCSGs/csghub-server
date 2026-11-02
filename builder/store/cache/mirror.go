package cache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const (
	// MirrorMetricsRetention bounds the lifetime of worker snapshots.
	MirrorMetricsRetention = 180 * time.Second
	// MirrorMetricsQueueRetention avoids repeating aggregate SQL on every poll.
	MirrorMetricsQueueRetention = 5 * time.Second
	// MirrorMetricsSchemaVersion identifies the snapshot format.
	MirrorMetricsSchemaVersion = 2
	// mirrorMetricsBatchSize bounds each pipeline of instance reads.
	mirrorMetricsBatchSize = 200
)

// MirrorMetricsStore provides ordinary Redis commands for ephemeral snapshots.
type MirrorMetricsStore struct {
	client RedisClient
	prefix string
	close  func() error
}

// MirrorMetricsReadSnapshot contains only current worker and queue snapshots.
type MirrorMetricsReadSnapshot struct {
	NowMS     int64
	Instances []types.MirrorMetricsSnapshot
	Control   *types.MirrorMetricsControl
	Partial   bool
}

// NewMirrorMetricsStore wraps an existing Redis connection with an isolated cluster hash tag.
func NewMirrorMetricsStore(client RedisClient, scope string) *MirrorMetricsStore {
	if scope == "" {
		scope = "default"
	}
	scope = base64.RawURLEncoding.EncodeToString([]byte(scope))
	return &MirrorMetricsStore{client: client, prefix: "csghub:mirror_metrics:v2:{" + scope + "}:"}
}

// NewMirrorMetricsStoreFromConfig creates a bounded, reconnecting client without a blocking startup Ping.
func NewMirrorMetricsStoreFromConfig(cfg *config.Config) *MirrorMetricsStore {
	opts := &redis.Options{Addr: cfg.Redis.Endpoint, Username: cfg.Redis.User, Password: cfg.Redis.Password,
		MaxRetries: -1, DialTimeout: 200 * time.Millisecond, ReadTimeout: 200 * time.Millisecond, WriteTimeout: 200 * time.Millisecond, ContextTimeoutEnabled: true}
	client := redis.NewClient(opts)
	if cfg.Redis.SentinelMode {
		_ = client.Close()
		client = redis.NewFailoverClient(&redis.FailoverOptions{MasterName: cfg.Redis.SentinelMaster,
			SentinelAddrs: strings.Split(cfg.Redis.SentinelEndpoint, ","), Username: cfg.Redis.User, Password: cfg.Redis.Password,
			MaxRetries: -1, DialTimeout: 200 * time.Millisecond, ReadTimeout: 200 * time.Millisecond, WriteTimeout: 200 * time.Millisecond, ContextTimeoutEnabled: true})
	}
	store := NewMirrorMetricsStore(NewCacheWithClient(context.Background(), client), cfg.Mirror.Metrics.Scope)
	store.close = client.Close
	return store
}

// Close releases only a connection owned by this store.
func (s *MirrorMetricsStore) Close() error {
	if s != nil && s.close != nil {
		return s.close()
	}
	return nil
}

// PublishSnapshot overwrites the current instance value and refreshes its TTL.
// Index maintenance is best effort; the next tick repairs partial pipeline writes.
func (s *MirrorMetricsStore) PublishSnapshot(ctx context.Context, snapshot types.MirrorMetricsSnapshot) error {
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	_, err = s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, s.prefix+"instance:"+snapshot.Instance.InstanceID, encoded, MirrorMetricsRetention)
		pipe.ZAdd(ctx, s.prefix+"instances", redis.Z{Score: float64(now), Member: snapshot.Instance.InstanceID})
		pipe.ZRemRangeByScore(ctx, s.prefix+"instances", "-inf", strconv.FormatInt(now-MirrorMetricsRetention.Milliseconds(), 10))
		pipe.Expire(ctx, s.prefix+"instances", MirrorMetricsRetention)
		return nil
	})
	return err
}

// PublishControl caches queue counts without coordination between API replicas.
func (s *MirrorMetricsStore) PublishControl(ctx context.Context, control *types.MirrorMetricsControl) error {
	encoded, err := json.Marshal(control)
	if err != nil {
		return err
	}
	return s.client.SetEx(ctx, s.prefix+"queues", string(encoded), MirrorMetricsQueueRetention)
}

// Read loads the current queue value and the fixed instance index, then batches GETs.
// Reads never extend TTLs. Missing or malformed snapshots are skipped.
func (s *MirrorMetricsStore) Read(ctx context.Context) (*MirrorMetricsReadSnapshot, error) {
	result := &MirrorMetricsReadSnapshot{NowMS: time.Now().UnixMilli()}
	var control *redis.StringCmd
	var index *redis.StringSliceCmd
	_, err := s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		control = pipe.Get(ctx, s.prefix+"queues")
		index = pipe.ZRangeByScore(ctx, s.prefix+"instances", &redis.ZRangeBy{Min: strconv.FormatInt(result.NowMS-MirrorMetricsRetention.Milliseconds(), 10), Max: "+inf"})
		return nil
	})
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	// A queue cache miss can hide a later command error in the pipeline result.
	if err := control.Err(); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if err := index.Err(); err != nil {
		return nil, err
	}
	if data, err := control.Result(); err == nil {
		var value types.MirrorMetricsControl
		if json.Unmarshal([]byte(data), &value) == nil && value.SchemaVersion == MirrorMetricsSchemaVersion {
			result.Control = &value
		} else {
			result.Partial = true
		}
	}
	ids := index.Val()
	for start := 0; start < len(ids); start += mirrorMetricsBatchSize {
		if err := s.readInstanceBatch(ctx, result, ids[start:min(start+mirrorMetricsBatchSize, len(ids))]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// readInstanceBatch bounds Redis commands while retaining cluster-wide totals.
func (s *MirrorMetricsStore) readInstanceBatch(ctx context.Context, result *MirrorMetricsReadSnapshot, ids []string) error {
	commands := make([]*redis.StringCmd, len(ids))
	_, err := s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i, id := range ids {
			commands[i] = pipe.Get(ctx, s.prefix+"instance:"+id)
		}
		return nil
	})
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	for i, command := range commands {
		// Missing keys are expected; other GET errors must reach the API.
		if err := command.Err(); err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		var snapshot types.MirrorMetricsSnapshot
		if command.Err() != nil || json.Unmarshal([]byte(command.Val()), &snapshot) != nil || snapshot.SchemaVersion != MirrorMetricsSchemaVersion || snapshot.Instance.InstanceID != ids[i] {
			result.Partial = true
			continue
		}
		result.Instances = append(result.Instances, snapshot)
	}
	return nil
}
