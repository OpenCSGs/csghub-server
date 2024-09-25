package mirror

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/queue"
)

type MirrorPriorityQueue struct {
	mq         *queue.PriorityQueue
	tasks      chan queue.MirrorTask
	numWorkers int
}

func NewMirrorPriorityQueue(config *config.Config) (*MirrorPriorityQueue, error) {
	s := &MirrorPriorityQueue{}
	mq, err := queue.GetPriorityQueueInstance()
	if err != nil {
		return nil, fmt.Errorf("fail to get priority queue: %w", err)
	}
	s.mq = mq
	s.tasks = make(chan queue.MirrorTask)
	s.numWorkers = config.Mirror.WorkerNumber
	return s, nil
}

func (ms *MirrorPriorityQueue) EnqueueMirrorTasks() {
	mirrorStore := database.NewMirrorStore()
	mirrors, err := mirrorStore.ToSyncRepo(context.Background())
	if err != nil {
		slog.Error("fail to get mirror to sync", slog.String("error", err.Error()))
		return
	}

	for _, mirror := range mirrors {
		ms.mq.PushRepoMirror(&queue.MirrorTask{
			MirrorID:  mirror.ID,
			Priority:  queue.Priority(mirror.Priority),
			CreatedAt: mirror.CreatedAt.Unix(),
		})
		mirror.Status = types.MirrorWaiting
		err = mirrorStore.Update(context.Background(), &mirror)
		if err != nil {
			slog.Error("fail to update mirror status", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
			continue
		}
	}

	mirrors, err = mirrorStore.ToSyncLfs(context.Background())
	if err != nil {
		slog.Error("fail to get mirror to sync", slog.String("error", err.Error()))
		return
	}

	for _, mirror := range mirrors {
		ms.mq.PushLfsMirror(&queue.MirrorTask{
			MirrorID:  mirror.ID,
			Priority:  queue.Priority(mirror.Priority),
			CreatedAt: mirror.CreatedAt.Unix(),
		})
		mirror.Status = types.MirrorWaiting
		err = mirrorStore.Update(context.Background(), &mirror)
		if err != nil {
			slog.Error("fail to update mirror status", slog.Int64("mirrorId", mirror.ID), slog.String("error", err.Error()))
			continue
		}
	}
}
