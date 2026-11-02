package component

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestMirrorComponentDeleteClearsRepoSyncCache(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	jobClient := useFakeMirrorJobClient(mc)
	syncCache := mockcache.NewMockCache(t)
	mc.syncCache = syncCache
	mc.config.Mirror.PartSize = 64

	mirror := &database.Mirror{ID: 321, RepositoryID: 123}
	mc.mocks.stores.MirrorMock().EXPECT().FindByID(ctx, mirror.ID).Return(mirror, nil)
	mc.mocks.stores.MirrorMock().EXPECT().DeleteWithTaskCancelTx(ctx, mirror.ID, jobClient).Return(nil)
	syncCache.EXPECT().DeleteRepoSyncCache(ctx, mirror.RepositoryID).Return(nil)

	require.NoError(t, mc.Delete(ctx, mirror.ID))
}

func TestMirrorComponent_Repos(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: ""}, true).Return([]database.Mirror{{
		CurrentTask: &database.MirrorTask{ID: 123, Progress: 100, Status: types.MirrorLfsSyncFinished},
		Progress:    100, Repository: &database.Repository{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepositoryType: types.ModelRepo},
	}}, 100, nil)

	data, total, err := mc.Repos(ctx, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.MirrorRepo{{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepoType: types.ModelRepo, Progress: 100, TaskID: 123}}, data)
}

func TestMirrorComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: "foo"}, false).Return(
		[]database.Mirror{{CurrentTask: &database.MirrorTask{Status: types.MirrorLfsSyncFinished}, Username: "user", LastMessage: "msg", Repository: &database.Repository{}}}, 100, nil,
	)

	data, total, err := mc.Index(ctx, 10, 1, types.MirrorFilter{Search: "foo"})
	require.NoError(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Mirror{{Username: "user", LastMessage: "msg", LocalRepoPath: "s/", Status: types.MirrorLfsSyncFinished}}, data)
}

func TestMirrorComponent_Statistic(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	mc.mocks.stores.MirrorMock().EXPECT().StatusCount(ctx).Return([]database.MirrorStatusCount{{Status: types.MirrorRepoSyncFinished, Count: 100}}, nil)

	status, err := mc.Statistics(ctx)
	require.NoError(t, err)
	require.Equal(t, []types.MirrorStatusCount{{Status: types.MirrorRepoSyncFinished, Count: 100}}, status)
}

func TestInferMirrorRepositoryTypeFromPath(t *testing.T) {
	tests := []struct {
		path string
		want types.RepositoryType
	}{
		{path: "/acme/model", want: types.ModelRepo},
		{path: "/datasets/acme/data", want: types.DatasetRepo},
		{path: "/codes/acme/project", want: types.CodeRepo},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			require.Equal(t, tt.want, inferMirrorRepositoryTypeFromPath(tt.path))
		})
	}
}

// metricsTestQuery supplies transport defaults.
func metricsTestQuery() types.MirrorMetricsQuery {
	return types.MirrorMetricsQuery{Service: "all", Page: 1, Per: 20, InstancePage: 1, InstancePer: 20}
}

// TestMirrorMetricsSnapshotAggregation verifies global sums and expiration without task reconciliation.
func TestMirrorMetricsSnapshotAggregation(t *testing.T) {
	now := int64(1000000)
	worker := func(id, stage string, at int64) types.MirrorMetricsSnapshot {
		return types.MirrorMetricsSnapshot{SchemaVersion: 2, Instance: types.MirrorMetricsInstance{InstanceID: id, Service: stage, RuntimeState: "ready", SampledAtMS: &at, DownloadBPS: metricsPointer(100.0), UploadBPS: metricsPointer(30.0)}, Executions: []types.MirrorMetricsTask{{TaskID: id, RepoPath: "models/a/b"}}}
	}
	snapshot := &cache.MirrorMetricsReadSnapshot{NowMS: now, Control: &types.MirrorMetricsControl{SampledAtMS: now, Queues: types.MirrorMetricsQueues{Repo: types.MirrorMetricsQueue{Waiting: 30000}}}, Instances: []types.MirrorMetricsSnapshot{worker("1", "lfs", now), worker("2", "lfs", now), worker("3", "repo", now-4000), worker("4", "lfs", now-11000)}}
	q := metricsTestQuery()
	q.InstanceID = "1"
	response := aggregateMirrorMetrics(snapshot, q)
	require.Equal(t, 2, response.Overview.OnlineInstances.LFS)
	require.Equal(t, 1, response.Overview.OnlineInstances.Repo)
	require.Equal(t, 2, response.Overview.ExecutingTasks.LFS)
	require.Equal(t, 30000, response.Overview.Queues.Repo.Waiting)
	require.Equal(t, 200.0, *response.Overview.Throughput.ClusterDownloadBPS)
	require.Equal(t, 60.0, *response.Overview.Throughput.ClusterUploadBPS)
	require.Equal(t, now+6000, response.Overview.CountsValidUntilMS.Repo)
	require.Equal(t, now+10000, response.Overview.CountsValidUntilMS.LFS)
	require.Len(t, response.Instances.Items, 1)
	require.Len(t, response.Tasks.Items, 1)
	snapshot.Instances[0].Instance.SampledAtMS = metricsPointer(now - 4000)
	q = metricsTestQuery()
	response = aggregateMirrorMetrics(snapshot, q)
	require.Nil(t, response.Overview.Throughput.ClusterDownloadBPS)
	require.Nil(t, response.Overview.Throughput.ClusterUploadBPS)
	require.Equal(t, now-4000, *response.Overview.Throughput.SampledAtMS)
	require.Nil(t, response.Tasks.Items[0].DownloadBPS)
	snapshot.Instances = nil
	response = aggregateMirrorMetrics(snapshot, q)
	require.Zero(t, *response.Overview.Throughput.ClusterDownloadBPS)
	require.Empty(t, response.Tasks.Items)
}

// TestMirrorMetricsThroughputUnknownDirections verifies independent, order-insensitive unknown propagation.
func TestMirrorMetricsThroughputUnknownDirections(t *testing.T) {
	now := int64(1000000)
	worker := func(down, up *float64) types.MirrorMetricsInstance {
		return types.MirrorMetricsInstance{Service: "lfs", Freshness: "fresh", RuntimeState: "ready",
			SampledAtMS: metricsPointer(now - 1000), DownloadBPS: down, UploadBPS: up}
	}
	a := worker(metricsPointer(10.0), metricsPointer(20.0))
	b := worker(metricsPointer(30.0), metricsPointer(40.0))
	missingDown := worker(nil, b.UploadBPS)
	missingUp := worker(b.DownloadBPS, nil)
	missingBoth := worker(nil, nil)
	zero := worker(metricsPointer(0.0), metricsPointer(0.0))
	stopping := missingDown
	stopping.RuntimeState = "stopping"
	stopped := missingBoth
	stopped.RuntimeState = "stopped"
	offline := missingBoth
	offline.Freshness = "stale"
	repo := missingBoth
	repo.Service = "repo"
	for _, tc := range []struct {
		name          string
		instances     []types.MirrorMetricsInstance
		down, up      *float64
		noParticipant bool
	}{
		{name: "known totals", instances: []types.MirrorMetricsInstance{a, b}, down: metricsPointer(40.0), up: metricsPointer(60.0)},
		{name: "unknown download first", instances: []types.MirrorMetricsInstance{missingDown, a}, up: metricsPointer(60.0)},
		{name: "unknown download last", instances: []types.MirrorMetricsInstance{a, missingDown}, up: metricsPointer(60.0)},
		{name: "unknown upload first", instances: []types.MirrorMetricsInstance{missingUp, a}, down: metricsPointer(40.0)},
		{name: "unknown upload last", instances: []types.MirrorMetricsInstance{a, missingUp}, down: metricsPointer(40.0)},
		{name: "different unknown directions", instances: []types.MirrorMetricsInstance{missingDown, missingUp}},
		{name: "all unknown", instances: []types.MirrorMetricsInstance{missingBoth}},
		{name: "unknown and observed zero", instances: []types.MirrorMetricsInstance{zero, missingBoth}},
		{name: "observed zeros", instances: []types.MirrorMetricsInstance{zero, zero}, down: metricsPointer(0.0), up: metricsPointer(0.0)},
		{name: "stopping participates", instances: []types.MirrorMetricsInstance{a, stopping}, up: metricsPointer(60.0)},
		{name: "stopped ignored", instances: []types.MirrorMetricsInstance{a, stopped}, down: a.DownloadBPS, up: a.UploadBPS},
		{name: "offline ignored", instances: []types.MirrorMetricsInstance{a, offline}, down: a.DownloadBPS, up: a.UploadBPS},
		{name: "repo ignored", instances: []types.MirrorMetricsInstance{a, repo}, down: a.DownloadBPS, up: a.UploadBPS},
		{name: "only excluded instances", instances: []types.MirrorMetricsInstance{stopped, offline, repo}, down: metricsPointer(0.0), up: metricsPointer(0.0), noParticipant: true},
		{name: "empty cluster", down: metricsPointer(0.0), up: metricsPointer(0.0), noParticipant: true},
		{name: "negative download", instances: []types.MirrorMetricsInstance{worker(metricsPointer(-1.0), b.UploadBPS), a}, up: metricsPointer(60.0)},
		{name: "nan download", instances: []types.MirrorMetricsInstance{worker(metricsPointer(math.NaN()), b.UploadBPS), a}, up: metricsPointer(60.0)},
		{name: "infinite upload", instances: []types.MirrorMetricsInstance{a, worker(b.DownloadBPS, metricsPointer(math.Inf(1)))}, down: metricsPointer(40.0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := metricsThroughput(tc.instances, now)
			require.Equal(t, tc.down, got.ClusterDownloadBPS)
			require.Equal(t, tc.up, got.ClusterUploadBPS)
			at := now - 1000
			if tc.noParticipant {
				at = now
			}
			require.Equal(t, at, *got.SampledAtMS)
		})
	}
}

// TestMirrorMetricsSnapshotSpeedExpiry covers the online-but-unknown interval and subsequent recovery.
func TestMirrorMetricsSnapshotSpeedExpiry(t *testing.T) {
	now := int64(1000000)
	for _, tc := range []struct {
		name      string
		age       int64
		online    int
		down, up  *float64
		sampledAt int64
	}{
		{name: "speed deadline included", age: 3000, online: 2, down: metricsPointer(100.0), up: metricsPointer(40.0), sampledAt: now - 3000},
		{name: "speed expired", age: 3001, online: 2, sampledAt: now - 3001},
		{name: "fresh zero with unknown worker", age: 4000, online: 2, sampledAt: now - 4000},
		{name: "presence deadline included", age: 10000, online: 2, sampledAt: now - 10000},
		{name: "offline worker excluded", age: 10001, online: 1, down: metricsPointer(0.0), up: metricsPointer(0.0), sampledAt: now},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := &cache.MirrorMetricsReadSnapshot{NowMS: now, Control: &types.MirrorMetricsControl{SampledAtMS: now},
				Instances: []types.MirrorMetricsSnapshot{
					{Instance: types.MirrorMetricsInstance{InstanceID: "a", Service: "lfs", RuntimeState: "ready", SampledAtMS: metricsPointer(now), DownloadBPS: metricsPointer(0.0), UploadBPS: metricsPointer(0.0)}},
					{Instance: types.MirrorMetricsInstance{InstanceID: "b", Service: "lfs", RuntimeState: "ready", SampledAtMS: metricsPointer(now - tc.age), DownloadBPS: metricsPointer(100.0), UploadBPS: metricsPointer(40.0)}},
				}}
			got := aggregateMirrorMetrics(snapshot, metricsTestQuery())
			require.Equal(t, "ok", got.Status)
			require.Equal(t, tc.online, got.Overview.OnlineInstances.LFS)
			require.Equal(t, tc.down, got.Overview.Throughput.ClusterDownloadBPS)
			require.Equal(t, tc.up, got.Overview.Throughput.ClusterUploadBPS)
			require.Equal(t, tc.sampledAt, *got.Overview.Throughput.SampledAtMS)
			// A fresh next snapshot restores totals without carrying the previous unknown state.
			snapshot.Instances[1].Instance.SampledAtMS = metricsPointer(now)
			got = aggregateMirrorMetrics(snapshot, metricsTestQuery())
			require.Equal(t, 2, got.Overview.OnlineInstances.LFS)
			require.Equal(t, 100.0, *got.Overview.Throughput.ClusterDownloadBPS)
			require.Equal(t, 40.0, *got.Overview.Throughput.ClusterUploadBPS)
		})
	}
}

// TestMirrorMetricsChartWorkers keeps all LFS chart choices despite table filters and pagination.
func TestMirrorMetricsChartWorkers(t *testing.T) {
	now := int64(1000000)
	snapshot := &cache.MirrorMetricsReadSnapshot{NowMS: now}
	for i := 24; i >= 0; i-- {
		id := fmt.Sprintf("lfs-%02d", i)
		snapshot.Instances = append(snapshot.Instances, types.MirrorMetricsSnapshot{Instance: types.MirrorMetricsInstance{
			InstanceID: id, Name: "shared-host", Service: "lfs", RuntimeState: "ready", SampledAtMS: metricsPointer(now),
			DownloadBPS: metricsPointer(float64(i)), UploadBPS: metricsPointer(float64(i * 2)),
		}})
	}
	snapshot.Instances = append(snapshot.Instances, types.MirrorMetricsSnapshot{Instance: types.MirrorMetricsInstance{InstanceID: "repo", Service: "repo", RuntimeState: "ready", SampledAtMS: metricsPointer(now)}})
	q := metricsTestQuery()
	q.InstancePer = 1
	q.InstancePage = 2
	response := aggregateMirrorMetrics(snapshot, q)
	require.Len(t, response.Instances.Items, 1)
	require.Len(t, response.LFSWorkers, 25)
	require.Equal(t, "lfs-00", response.LFSWorkers[0].InstanceID)
	require.Equal(t, "lfs-24", response.LFSWorkers[24].InstanceID)
	require.Equal(t, "shared-host", response.LFSWorkers[24].Name)
	require.Equal(t, 24.0, *response.LFSWorkers[24].DownloadBPS)
	require.Equal(t, 48.0, *response.LFSWorkers[24].UploadBPS)
	q.Service, q.InstanceID = "repo", "repo"
	response = aggregateMirrorMetrics(snapshot, q)
	require.Empty(t, response.Instances.Items)
	require.Len(t, response.LFSWorkers, 25)
	snapshot.Instances[0].Instance.SampledAtMS = metricsPointer(now - 4000)
	snapshot.Instances[1].Instance.RuntimeState = "stopped"
	response = aggregateMirrorMetrics(snapshot, q)
	require.Nil(t, response.LFSWorkers[24].DownloadBPS)
	require.Nil(t, response.LFSWorkers[24].UploadBPS)
	require.Nil(t, response.LFSWorkers[23].DownloadBPS)
	snapshot.Instances = nil
	response = aggregateMirrorMetrics(snapshot, q)
	require.NotNil(t, response.LFSWorkers)
	require.Empty(t, response.LFSWorkers)
}

// TestMirrorMetricsQueueLoading reuses cache and preserves worker metrics when SQL or cache writes fail.
func TestMirrorMetricsQueueLoading(t *testing.T) {
	snapshot := &cache.MirrorMetricsReadSnapshot{NowMS: time.Now().UnixMilli()}
	calls := 0
	c := &MirrorMetricsComponent{enabled: true, read: func(context.Context) (*cache.MirrorMetricsReadSnapshot, error) { return snapshot, nil }, queues: func(ctx context.Context) (types.MirrorMetricsQueues, error) {
		calls++
		_, ok := ctx.Deadline()
		require.True(t, ok)
		return types.MirrorMetricsQueues{LFS: types.MirrorMetricsQueue{Waiting: 7}}, nil
	}, saveQueues: func(context.Context, *types.MirrorMetricsControl) error { return nil }}
	response, err := c.Get(context.Background(), metricsTestQuery())
	require.NoError(t, err)
	require.Equal(t, 7, response.Overview.Queues.LFS.Waiting)
	_, err = c.Get(context.Background(), metricsTestQuery())
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	snapshot.Control = nil
	c.queues = func(context.Context) (types.MirrorMetricsQueues, error) {
		return types.MirrorMetricsQueues{}, errors.New("db down")
	}
	response, err = c.Get(context.Background(), metricsTestQuery())
	require.NoError(t, err)
	require.Equal(t, "partial", response.Status)
	require.Nil(t, response.Overview.Queues)
	c.enabled = false
	response, err = c.Get(context.Background(), metricsTestQuery())
	require.NoError(t, err)
	require.Equal(t, "disabled", response.Status)
}

// TestMirrorMetricsReadFailure skips SQL fallback when Redis cannot read the instance index.
func TestMirrorMetricsReadFailure(t *testing.T) {
	readError := errors.New("WRONGTYPE instance index")
	c := &MirrorMetricsComponent{
		enabled: true,
		read: func(context.Context) (*cache.MirrorMetricsReadSnapshot, error) {
			return nil, readError
		},
		queues: func(context.Context) (types.MirrorMetricsQueues, error) {
			t.Fatal("Redis read failure must not load queue counts")
			return types.MirrorMetricsQueues{}, nil
		},
		saveQueues: func(context.Context, *types.MirrorMetricsControl) error {
			t.Fatal("Redis read failure must not publish queue counts")
			return nil
		},
	}
	response, err := c.Get(context.Background(), metricsTestQuery())
	require.ErrorIs(t, err, readError)
	require.Nil(t, response)
}

// mirrorMetricsTestReporter isolates sampling from Redis and timer scheduling.
func mirrorMetricsTestReporter(now time.Time) *MirrorMetricsReporter {
	return &MirrorMetricsReporter{instance: types.MirrorMetricsInstance{InstanceID: "test", Service: "lfs", RuntimeState: "ready"}, executions: make(map[*mirrorMetricsExecution]bool), lastSample: now}
}

// mirrorMetricsTestTerminalBody forwards bytes together with EOF and records Close calls.
type mirrorMetricsTestTerminalBody struct{ closed bool }

// Read simulates a legal terminal body read.
func (b *mirrorMetricsTestTerminalBody) Read(p []byte) (int, error) { return copy(p, "abc"), io.EOF }

// Close records cleanup of the original stream.
func (b *mirrorMetricsTestTerminalBody) Close() error { b.closed = true; return nil }

// TestMirrorMetricsReporterCurrentSnapshot excludes unadmitted work and removes released work.
func TestMirrorMetricsReporterCurrentSnapshot(t *testing.T) {
	start := time.Now()
	r := mirrorMetricsTestReporter(start)
	ctx, release := r.Track(context.Background(), types.MirrorMetricsTask{TaskID: "1", Stage: "lfs"})
	require.Empty(t, r.capture(start).Executions)
	entry := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	entry.previousSample = start.Add(-time.Hour)
	MirrorMetricsRunning(ctx, types.MirrorMetricsTask{RepoPath: "models/a/b", StartedAtMS: metricsPointer(start.UnixMilli())})
	require.False(t, entry.previousSample.Before(start), "admission waiting time must not dilute the first transfer rate")
	MirrorMetricsOperation(ctx, "transfer")
	body := &mirrorMetricsTestTerminalBody{}
	n, err := MirrorMetricsWrapBody(ctx, body).Read(make([]byte, 8))
	require.Equal(t, 3, n)
	require.ErrorIs(t, err, io.EOF)
	require.NoError(t, MirrorMetricsWrapBody(ctx, body).Close())
	require.True(t, body.closed)
	snapshot := r.capture(start.Add(time.Second))
	require.Len(t, snapshot.Executions, 1)
	require.Equal(t, "transfer", *snapshot.Executions[0].Operation)
	require.Equal(t, "3", *snapshot.Instance.DownloadedBytes)
	release()
	release()
	require.Empty(t, r.capture(start.Add(2*time.Second)).Executions)
}

// TestMirrorMetricsReporterConcurrentDuplexAndProgress keeps actual network bytes separate from verified progress.
func TestMirrorMetricsReporterConcurrentDuplexAndProgress(t *testing.T) {
	start := time.Now()
	r := mirrorMetricsTestReporter(start)
	ctx, release := r.Track(context.Background(), types.MirrorMetricsTask{Stage: "lfs"})
	defer release()
	MirrorMetricsRunning(ctx, types.MirrorMetricsTask{})
	require.Nil(t, r.capture(start).Executions[0].LFS)
	MirrorMetricsInitProgress(ctx, 100, 20, 2, 1)
	MirrorMetricsAddPart(ctx, "oid", 80, 30)
	MirrorMetricsAddPart(ctx, "oid", 80, 100)
	var group sync.WaitGroup
	entry := ctx.Value(mirrorMetricsExecutionKey{}).(*mirrorMetricsExecution)
	for range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, _ = io.Copy(io.Discard, MirrorMetricsWrapBody(ctx, io.NopCloser(strings.NewReader("abcdefghij"))))
			_, _ = io.Copy(io.Discard, &mirrorMetricsUploadBody{ReadCloser: io.NopCloser(strings.NewReader("abc")), execution: entry})
		}()
	}
	group.Wait()
	snapshot := r.capture(start.Add(time.Second))
	require.InDelta(t, 200, *snapshot.Instance.DownloadBPS, .001)
	require.InDelta(t, 60, *snapshot.Instance.UploadBPS, .001)
	require.Equal(t, "80", *snapshot.Executions[0].LFS.ConfirmedPartBytes)
	require.Equal(t, float64(99), *snapshot.Executions[0].LFS.ProgressPercent)
	MirrorMetricsResetObject(ctx, "oid")
	require.Equal(t, "0", *r.capture(start.Add(2 * time.Second)).Executions[0].LFS.ConfirmedPartBytes)
	MirrorMetricsCompleteObject(ctx, "oid", 80)
	MirrorMetricsCompleteObject(ctx, "oid", 80)
	MirrorMetricsAddPart(ctx, "oid", 80, 20)
	snapshot = r.capture(start.Add(3 * time.Second))
	require.Equal(t, "100", *snapshot.Executions[0].LFS.CompleteBytes)
	require.Equal(t, 2, *snapshot.Executions[0].LFS.FilesDone)
	require.Equal(t, float64(100), *snapshot.Executions[0].LFS.ProgressPercent)
}

// TestMirrorMetricsReporterPeriodicRedis verifies timer publication, recovery and a bounded final stop.
func TestMirrorMetricsReporterPeriodicRedis(t *testing.T) {
	server := miniredis.RunT(t)
	cfg := &config.Config{}
	cfg.Redis.Endpoint = server.Addr()
	cfg.Mirror.Metrics.Enabled = true
	cfg.Mirror.WorkerNumber = 4
	r := NewMirrorMetricsReporter(cfg, "lfs")
	r.SetState("ready")
	store := cache.NewMirrorMetricsStoreFromConfig(cfg)
	defer store.Close()
	defer r.Stop(context.Background())
	ctx, release := r.Track(context.Background(), types.MirrorMetricsTask{TaskID: "1", Stage: "lfs"})
	MirrorMetricsRunning(ctx, types.MirrorMetricsTask{})
	_, err := io.Copy(io.Discard, MirrorMetricsWrapBody(ctx, io.NopCloser(strings.NewReader("download"))))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		value, err := store.Read(context.Background())
		return err == nil && len(value.Instances) == 1 && len(value.Instances[0].Executions) == 1
	}, 2500*time.Millisecond, 10*time.Millisecond)
	release()
	server.SetError("ERR unavailable")
	// Business instrumentation remains usable while Redis refuses commands.
	ctx, release = r.Track(context.Background(), types.MirrorMetricsTask{TaskID: "2", Stage: "lfs"})
	MirrorMetricsRunning(ctx, types.MirrorMetricsTask{})
	release()
	server.SetError("")
	shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r.Stop(shutdown)
	value, err := store.Read(context.Background())
	require.NoError(t, err)
	require.Len(t, value.Instances, 1)
	require.Equal(t, "stopped", value.Instances[0].Instance.RuntimeState)
	require.Empty(t, value.Instances[0].Executions)
	require.Equal(t, "8", *value.Instances[0].Instance.DownloadedBytes)
}

// mirrorMetricsTestRoundTripFunc exercises the real SDK at its HTTP boundary without a server.
type mirrorMetricsTestRoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip delegates request consumption to a deterministic test transport.
func (f mirrorMetricsTestRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// TestMirrorMetricsUploadTransportSDKBuffering verifies both client constructors install the
// observer after SDK buffering, and multipart and small uploads stay independent.
func TestMirrorMetricsUploadTransportSDKBuffering(t *testing.T) {
	for _, multipart := range []bool{false, true} {
		t.Run(map[bool]string{false: "small", true: "multipart"}[multipart], func(t *testing.T) {
			reporter := mirrorMetricsTestReporter(time.Now())
			ctx, done := reporter.Track(context.Background(), types.MirrorMetricsTask{TaskID: "1", Stage: "lfs"})
			defer done()
			transport := MirrorMetricsUploadTransport(mirrorMetricsTestRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodPut, request.Method)
				require.Zero(t, reporter.totalUploadedBytes.Load(), "SDK pre-reading is not upload traffic")
				_, err := io.Copy(io.Discard, request.Body)
				require.NoError(t, err)
				require.NoError(t, request.Body.Close())
				return &http.Response{StatusCode: 200, Header: http.Header{"Etag": {`"part-etag"`}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
			}))
			cfg := &config.Config{}
			cfg.S3.Endpoint, cfg.S3.Region, cfg.S3.BucketLookup = "storage.example.com", "us-east-1", "path"
			body := MirrorMetricsWrapBody(ctx, io.NopCloser(strings.NewReader("abcdef")))
			if multipart {
				core, err := s3.NewMinioCore(cfg, s3.WithTransport(transport))
				require.NoError(t, err)
				_, err = core.PutObjectPart(ctx, "test-bucket", "object", "upload", 1, body, 6, minio.PutObjectPartOptions{DisableContentSha256: true})
				require.NoError(t, err)
			} else {
				client, err := s3.NewMinio(cfg, s3.WithTransport(transport))
				require.NoError(t, err)
				_, err = client.PutObject(ctx, "test-bucket", "object", body, 6, minio.PutObjectOptions{})
				require.NoError(t, err)
			}
			require.Equal(t, int64(6), reporter.totalBytes.Load())
			require.Equal(t, int64(6), reporter.totalUploadedBytes.Load())
		})
	}
}

// TestMirrorMetricsUploadTransportPartialAndReplay counts partial failures and independent
// replays while preserving the original request and forwarded close/error behavior.
func TestMirrorMetricsUploadTransportPartialAndReplay(t *testing.T) {
	reporter := mirrorMetricsTestReporter(time.Now())
	ctx, done := reporter.Track(context.Background(), types.MirrorMetricsTask{Stage: "lfs"})
	defer done()
	body := &mirrorMetricsTestTerminalBody{}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://storage/object", body)
	require.NoError(t, err)
	request.GetBody = func() (io.ReadCloser, error) { return &mirrorMetricsTestTerminalBody{}, nil }
	failure := errors.New("upload connection closed")
	transport := MirrorMetricsUploadTransport(mirrorMetricsTestRoundTripFunc(func(cloned *http.Request) (*http.Response, error) {
		require.NotSame(t, request, cloned)
		n, readErr := cloned.Body.Read(make([]byte, 10))
		require.Equal(t, 3, n)
		require.ErrorIs(t, readErr, io.EOF)
		require.NoError(t, cloned.Body.Close())
		replay, replayErr := cloned.GetBody()
		require.NoError(t, replayErr)
		_, readErr = replay.Read(make([]byte, 10))
		require.ErrorIs(t, readErr, io.EOF)
		require.NoError(t, replay.Close())
		return nil, failure
	}))
	response, err := transport.RoundTrip(request)
	if response != nil {
		require.NoError(t, response.Body.Close())
	}
	require.ErrorIs(t, err, failure)
	require.True(t, body.closed)
	require.Same(t, body, request.Body)
	require.Equal(t, int64(6), reporter.totalUploadedBytes.Load())
	require.Zero(t, reporter.totalBytes.Load(), "upload retries cannot invent source downloads")
}

// mirrorMetricsTestFailingRedis exposes a deterministic failed publication without relying on wall-clock sleeps.
type mirrorMetricsTestFailingRedis struct {
	cache.RedisClient
	fail   atomic.Bool
	failed chan struct{}
}

// Pipelined fails the first timer write and delegates all later ordinary commands.
func (c *mirrorMetricsTestFailingRedis) Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	if c.fail.Swap(false) {
		close(c.failed)
		return nil, errors.New("redis unavailable")
	}
	return c.RedisClient.Pipelined(ctx, fn)
}

// TestMirrorMetricsReporterRecoversRedis proves failed timer writes are discarded and the next tick recovers.
func TestMirrorMetricsReporterRecoversRedis(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	transport := &mirrorMetricsTestFailingRedis{RedisClient: cache.NewCacheWithClient(context.Background(), client), failed: make(chan struct{})}
	transport.fail.Store(true)
	store := cache.NewMirrorMetricsStore(transport, "test")
	r := newMirrorMetricsReporter(store, &config.Config{}, "lfs")
	r.SetState("ready")
	defer r.Stop(context.Background())
	ctx, release := r.Track(context.Background(), types.MirrorMetricsTask{TaskID: "1", Stage: "lfs"})
	defer release()
	MirrorMetricsRunning(ctx, types.MirrorMetricsTask{})
	_, err := io.Copy(io.Discard, MirrorMetricsWrapBody(ctx, io.NopCloser(strings.NewReader("still downloading"))))
	require.NoError(t, err)
	select {
	case <-transport.failed:
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("timer did not publish")
	}
	require.Eventually(t, func() bool {
		value, err := store.Read(context.Background())
		return err == nil && len(value.Instances) == 1 && value.Instances[0].Instance.DownloadedBytes != nil && *value.Instances[0].Instance.DownloadedBytes == "17"
	}, 2500*time.Millisecond, 10*time.Millisecond)
}
