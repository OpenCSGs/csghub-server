package cache

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
	"strings"
	"testing"
	"time"
)

// testStore isolates Redis for expiry and fault tests.
func testStore(t *testing.T) (*MirrorMetricsStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewMirrorMetricsStore(NewCacheWithClient(context.Background(), client), "test"), server
}

// TestMirrorMetricSnapshotOverwrite verifies plain overwrite, bounded keys and read-only TTL behavior.
func TestMirrorMetricSnapshotOverwrite(t *testing.T) {
	store, server := testStore(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	snapshot := types.MirrorMetricsSnapshot{SchemaVersion: MirrorMetricsSchemaVersion, Instance: types.MirrorMetricsInstance{InstanceID: "one", SampledAtMS: &now, Service: "lfs", ExecutingTasks: 1}, Executions: []types.MirrorMetricsTask{{TaskID: "1"}}}
	require.NoError(t, store.PublishSnapshot(ctx, snapshot))
	require.NoError(t, server.Set(store.prefix+"instance:one", "bad previous json"))
	snapshot.Executions = nil
	snapshot.Instance.ExecutingTasks = 0
	require.NoError(t, store.PublishSnapshot(ctx, snapshot))
	server.FastForward(time.Second)
	value, err := store.Read(ctx)
	require.NoError(t, err)
	require.Len(t, value.Instances, 1)
	require.Empty(t, value.Instances[0].Executions)
	require.Equal(t, MirrorMetricsRetention-time.Second, server.TTL(store.prefix+"instance:one"))
	require.Len(t, server.Keys(), 2)
	server.FastForward(MirrorMetricsRetention)
	value, err = store.Read(ctx)
	require.NoError(t, err)
	require.Empty(t, value.Instances)
}

// TestMirrorMetricQueueCache verifies a five-second summary without locks or event keys.
func TestMirrorMetricQueueCache(t *testing.T) {
	store, server := testStore(t)
	ctx := context.Background()
	control := &types.MirrorMetricsControl{SchemaVersion: MirrorMetricsSchemaVersion, Queues: types.MirrorMetricsQueues{Repo: types.MirrorMetricsQueue{Waiting: 30000}}}
	require.NoError(t, store.PublishControl(ctx, control))
	value, err := store.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, 30000, value.Control.Queues.Repo.Waiting)
	for _, key := range server.Keys() {
		require.True(t, strings.HasSuffix(key, ":queues"))
	}
	server.FastForward(MirrorMetricsQueueRetention)
	value, err = store.Read(ctx)
	require.NoError(t, err)
	require.Nil(t, value.Control)
}

// TestMirrorMetricReadFaults skips broken snapshots and surfaces transport errors.
func TestMirrorMetricReadFaults(t *testing.T) {
	store, server := testStore(t)
	ctx := context.Background()
	snapshot := types.MirrorMetricsSnapshot{SchemaVersion: MirrorMetricsSchemaVersion, Instance: types.MirrorMetricsInstance{InstanceID: "one"}}
	require.NoError(t, store.PublishSnapshot(ctx, snapshot))
	require.NoError(t, server.Set(store.prefix+"instance:one", "broken"))
	value, err := store.Read(ctx)
	require.NoError(t, err)
	require.True(t, value.Partial)
	require.Empty(t, value.Instances)
	// A successful next publication repairs the partial write with no replay mechanism.
	require.NoError(t, store.PublishSnapshot(ctx, snapshot))
	value, err = store.Read(ctx)
	require.NoError(t, err)
	require.False(t, value.Partial)
	server.SetError("ERR unavailable")
	_, err = store.Read(ctx)
	require.Error(t, err)
}

// TestMirrorMetricIndexReadFailure rejects index errors even when the first pipeline command misses.
func TestMirrorMetricIndexReadFailure(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "missing_queue"
		if expired {
			name = "expired_queue"
		}
		t.Run(name, func(t *testing.T) {
			store, server := testStore(t)
			ctx := context.Background()
			if expired {
				require.NoError(t, store.PublishControl(ctx, &types.MirrorMetricsControl{SchemaVersion: MirrorMetricsSchemaVersion}))
				server.FastForward(MirrorMetricsQueueRetention)
			}
			require.False(t, server.Exists(store.prefix+"queues"))
			require.NoError(t, server.Set(store.prefix+"instances", "wrong type"))
			value, err := store.Read(ctx)
			require.ErrorContains(t, err, "WRONGTYPE")
			require.Nil(t, value)

			// Removing the invalid index restores a valid empty read without a queue cache.
			server.Del(store.prefix + "instances")
			value, err = store.Read(ctx)
			require.NoError(t, err)
			require.Nil(t, value.Control)
			require.Empty(t, value.Instances)
			require.False(t, value.Partial)
		})
	}
}

// TestMirrorMetricInstanceReadFailure rejects GET errors regardless of missing-key ordering and allows recovery.
func TestMirrorMetricInstanceReadFailure(t *testing.T) {
	for _, missingFirst := range []bool{true, false} {
		name := "failure_first"
		if missingFirst {
			name = "missing_first"
		}
		t.Run(name, func(t *testing.T) {
			store, server := testStore(t)
			ctx := context.Background()
			for _, id := range []string{"a", "b"} {
				require.NoError(t, store.PublishSnapshot(ctx, types.MirrorMetricsSnapshot{
					SchemaVersion: MirrorMetricsSchemaVersion, Instance: types.MirrorMetricsInstance{InstanceID: id},
				}))
				server.Del(store.prefix + "instance:" + id)
			}
			failedID := "a"
			if missingFirst {
				failedID = "b"
			}
			server.HSet(store.prefix+"instance:"+failedID, "field", "wrong type")
			value, err := store.Read(ctx)
			require.ErrorContains(t, err, "WRONGTYPE")
			require.Nil(t, value)

			// An ordinary missing snapshot still permits a partial response after the failure is repaired.
			server.Del(store.prefix + "instance:" + failedID)
			require.NoError(t, store.PublishSnapshot(ctx, types.MirrorMetricsSnapshot{
				SchemaVersion: MirrorMetricsSchemaVersion, Instance: types.MirrorMetricsInstance{InstanceID: failedID},
			}))
			value, err = store.Read(ctx)
			require.NoError(t, err)
			require.True(t, value.Partial)
			require.Len(t, value.Instances, 1)
			require.Equal(t, failedID, value.Instances[0].Instance.InstanceID)
		})
	}
}
