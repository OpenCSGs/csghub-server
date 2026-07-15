package component

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	mirrorcache "opencsg.com/csghub-server/mirror/cache"
)

type ManagerComponent interface {
	Cancel(ctx context.Context, mirrorID int64) (bool, error)
	ListTasks(ctx context.Context, per, page int) (types.MirrorListResp, error)
}

type managerComponentImpl struct {
	mirrorTaskStore database.MirrorTaskJobStore
	jobClient       workhub.JobClient
	config          *config.Config
	// syncCache removes LFS upload cache when a mirror task is cancelled.
	syncCacheMu sync.Mutex
	syncCache   mirrorcache.Cache
	// partSize identifies the LFS multipart cache namespace used by current config.
	partSize string
}

func NewMirrorComponent(cfg *config.Config) (ManagerComponent, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	jobClient, err := workhub.NewJobClient(context.Background(), database.GetDB().BunDB)
	if err != nil {
		return nil, fmt.Errorf("fail to create mirror manager job client: %w", err)
	}
	return &managerComponentImpl{
		mirrorTaskStore: database.NewMirrorTaskJobStore(),
		jobClient:       jobClient,
		config:          cfg,
		partSize:        strconv.Itoa(cfg.Mirror.PartSize),
	}, nil
}

func (c *managerComponentImpl) Cancel(ctx context.Context, taskID int64) (bool, error) {
	task, err := c.mirrorTaskStore.FindByID(ctx, taskID)
	if err != nil {
		return false, fmt.Errorf("fail to find mirror task: %w", err)
	}

	var repoID int64
	if task != nil && task.Mirror != nil {
		repoID = task.Mirror.RepositoryID
	}

	dbCancelled, err := c.mirrorTaskStore.CancelMirrorTaskByIDWithJobCancel(ctx, taskID, c.jobClient)
	if err != nil {
		return false, fmt.Errorf("fail to cancel mirror task in db: %w", err)
	}

	if !dbCancelled {
		return false, fmt.Errorf("no task found for mirror %d", taskID)
	}

	c.deleteRepoSyncCache(ctx, repoID)

	return true, nil
}

// deleteRepoSyncCache removes LFS cache for the repository after a task cancel succeeds.
func (c *managerComponentImpl) deleteRepoSyncCache(ctx context.Context, repoID int64) {
	if repoID == 0 {
		return
	}
	syncCache := c.syncCache
	if syncCache == nil {
		c.syncCacheMu.Lock()
		if c.syncCache == nil && c.config != nil {
			cache, err := mirrorcache.NewCache(context.Background(), c.config)
			if err != nil {
				slog.WarnContext(ctx, "failed to create mirror sync cache for cleanup", slog.Any("error", err), slog.Int64("repo_id", repoID))
			} else {
				c.syncCache = cache
			}
		}
		syncCache = c.syncCache
		c.syncCacheMu.Unlock()
		if syncCache == nil {
			return
		}
	}
	if err := syncCache.DeleteRepoSyncCache(ctx, repoID, c.partSize); err != nil {
		slog.WarnContext(ctx, "failed to delete mirror task cache", slog.Any("error", err), slog.Int64("repo_id", repoID))
	}
}

func (c *managerComponentImpl) ListTasks(ctx context.Context, per, page int) (types.MirrorListResp, error) {
	var resp types.MirrorListResp
	runningTasks, err := c.mirrorTaskStore.ListByStatusWithPriority(
		ctx,
		[]types.MirrorTaskStatus{types.MirrorRepoSyncStart, types.MirrorLfsSyncStart},
		per,
		page,
	)
	if err != nil {
		return resp, fmt.Errorf("fail to list running tasks: %w", err)
	}
	for _, task := range runningTasks {
		if task.Mirror != nil && task.Mirror.Repository != nil {
			resp.RunningTasks = append(resp.RunningTasks, types.MirrorTask{
				MirrorID:  task.MirrorID,
				TaskID:    task.ID,
				SourceUrl: task.Mirror.SourceUrl,
				Priority:  int(task.Priority),
				RepoPath:  task.Mirror.RepoPath(),
			})
		}
	}

	waitingTasks, err := c.mirrorTaskStore.ListByStatusWithPriority(
		ctx,
		[]types.MirrorTaskStatus{
			types.MirrorQueued,
			types.MirrorRepoSyncFinished,
			types.MirrorRepoSyncFailed,
			types.MirrorLfsSyncFailed,
		},
		per,
		page,
	)
	if err != nil {
		return resp, fmt.Errorf("fail to list waiting tasks: %w", err)
	}
	for _, task := range waitingTasks {
		if task.Mirror != nil && task.Mirror.Repository != nil {
			resp.WaitingTasks = append(resp.WaitingTasks, types.MirrorTask{
				MirrorID:  task.MirrorID,
				TaskID:    task.ID,
				SourceUrl: task.Mirror.SourceUrl,
				Priority:  int(task.Priority),
				RepoPath:  task.Mirror.RepoPath(),
			})
		}
	}

	return resp, nil
}
