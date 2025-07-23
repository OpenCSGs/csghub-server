package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/manager"
)

type ManagerComponent interface {
	SyncNow(ctx context.Context, workerID int, mirrorTaskID int64) error
	Cancel(ctx context.Context, mirrorID int64) (bool, error)
	ListTasks(ctx context.Context, per, page int) (types.MirrorListResp, error)
}

type managerComponentImpl struct {
	mirrorStore     database.MirrorStore
	mirrorTaskStore database.MirrorTaskStore
	manager         *manager.Manager
}

func NewMirrorComponent(cfg *config.Config) (ManagerComponent, error) {
	m, err := manager.GetManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("fail to get manager: %w", err)
	}
	return &managerComponentImpl{
		mirrorStore:     database.NewMirrorStore(),
		mirrorTaskStore: database.NewMirrorTaskStore(),
		manager:         m,
	}, nil
}

func (c *managerComponentImpl) SyncNow(ctx context.Context, workerID int, mtID int64) error {
	if workerID == 0 {
		workerID = 1
	}
	mt, err := c.mirrorTaskStore.FindByID(ctx, mtID)
	if err != nil {
		return fmt.Errorf("fail to find mirror task: %w", err)
	}

	mt.Status = types.MirrorLfsSyncStart
	_, err = c.mirrorTaskStore.Update(ctx, *mt)
	if err != nil {
		return fmt.Errorf("fail to update mirror task: %w", err)
	}
	err = c.manager.ReRun(workerID, mt)
	if err != nil {
		return fmt.Errorf("fail to run worker: %w", err)
	}
	return nil
}

func (c *managerComponentImpl) Cancel(ctx context.Context, mirrorID int64) (bool, error) {
	found, err := c.manager.StopWorkerByMirrorID(mirrorID)
	if err != nil {
		return found, fmt.Errorf("fail to stop worker: %w", err)
	}
	return found, nil
}

func (c *managerComponentImpl) ListTasks(ctx context.Context, per, page int) (types.MirrorListResp, error) {
	var (
		resp     types.MirrorListResp
		lfsTasks []types.MirrorTask
	)
	taskResp := make(map[int]types.MirrorTask)
	tasks := c.manager.RunningTasks()
	for id, task := range tasks {
		if task.Mirror != nil && task.Mirror.Repository != nil {
			taskResp[id] = types.MirrorTask{
				MirrorID:  task.MirrorID,
				SourceUrl: task.Mirror.SourceUrl,
				Priority:  int(task.Priority),
				RepoPath:  task.Mirror.RepoPath(),
			}
		}
	}
	resp.RunningTasks = taskResp
	waittingTasks, err := c.mirrorTaskStore.ListByStatusWithPriority(
		ctx,
		[]types.MirrorTaskStatus{types.MirrorRepoSyncFinished},
		per,
		page,
	)
	if err != nil {
		return resp, fmt.Errorf("fail to list waitting tasks: %w", err)
	}
	for _, task := range waittingTasks {
		if task.Mirror != nil && task.Mirror.Repository != nil {
			lfsTasks = append(lfsTasks, types.MirrorTask{
				MirrorID:  task.ID,
				SourceUrl: task.Mirror.SourceUrl,
				Priority:  int(task.Priority),
				RepoPath:  task.Mirror.RepoPath(),
			})
		}
	}
	resp.LfsMirrorTasks = lfsTasks

	return resp, nil
}
