package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type Priority int

func (p Priority) Int() int { return int(p) }

const (
	HighPriority   Priority = 3
	MediumPriority Priority = 2
	LowPriority    Priority = 1
)

var PriorityMap = map[types.MirrorPriority]Priority{
	types.HighMirrorPriority:   HighPriority,
	types.MediumMirrorPriority: MediumPriority,
	types.LowMirrorPriority:    LowPriority,
}

const (
	repoQueueName = "repo_mirror_queue"
	lfsQueueName  = "lfs_mirror_queue"
)

type MirrorTask struct {
	MirrorID    int64    `json:"mirror_id"`
	Priority    Priority `json:"priority"`
	CreatedAt   int64    `json:"created_at"`
	MirrorToken string   `json:"mirror_token"`
}

type MirrorQueue struct {
	redis     cache.RedisClient
	QueueName string
}

func (m *MirrorTask) MarshalBinary() ([]byte, error) {
	return json.Marshal(m)
}

func (m *MirrorTask) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, m)
}

func (mq *MirrorQueue) Push(t *MirrorTask) {
	if t.CreatedAt == 0 {
		t.CreatedAt = time.Now().Unix()
	}
	_ = mq.redis.ZAdd(context.Background(), mq.QueueName, redis.Z{
		Score:  float64(t.CreatedAt) * float64(t.Priority),
		Member: t,
	})
}

func (mq *MirrorQueue) Pop() *MirrorTask {
	r, err := mq.redis.BZPopMax(context.Background(), mq.QueueName)
	if err != nil {
		return nil
	}
	var task MirrorTask
	_ = json.Unmarshal([]byte(r.Member.(string)), &task)
	return &task
}

type PriorityQueue interface {
	PushRepoMirror(mt *MirrorTask)
	PopRepoMirror() *MirrorTask
	PushLfsMirror(mt *MirrorTask)
	PopLfsMirror() *MirrorTask
}

type priorityQueueImpl struct {
	RepoMirrorQueue MirrorQueue
	LfsMirrorQueue  MirrorQueue
}

var (
	instance PriorityQueue
	once     sync.Once
	err      error
	c        *config.Config
)

func NewPriorityQueue(ctx context.Context, config *config.Config) (PriorityQueue, error) {
	redis, err := cache.NewCache(ctx, cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing redis: %w", err)
	}
	mq := &priorityQueueImpl{
		RepoMirrorQueue: MirrorQueue{
			redis:     redis,
			QueueName: repoQueueName,
		},
		LfsMirrorQueue: MirrorQueue{
			redis:     redis,
			QueueName: lfsQueueName,
		},
	}
	return mq, nil
}

func (pq *priorityQueueImpl) PushRepoMirror(mt *MirrorTask) {
	pq.RepoMirrorQueue.Push(mt)
}

func (pq *priorityQueueImpl) PopRepoMirror() *MirrorTask {
	return pq.RepoMirrorQueue.Pop()
}

func (pq *priorityQueueImpl) PushLfsMirror(mt *MirrorTask) {
	pq.LfsMirrorQueue.Push(mt)
}

func (pq *priorityQueueImpl) PopLfsMirror() *MirrorTask {
	return pq.LfsMirrorQueue.Pop()
}

func GetPriorityQueueInstance() (PriorityQueue, error) {
	once.Do(func() {
		c, err = config.LoadConfig()
		instance, err = NewPriorityQueue(context.Background(), c)
	})
	if err != nil {
		return nil, err
	}
	return instance, nil
}
