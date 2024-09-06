package queue

import (
	"container/heap"
	"sync"

	"opencsg.com/csghub-server/common/types"
)

type Priority int

func (p Priority) Int() int { return int(p) }

const (
	HighPriority   Priority = 0
	MediumPriority Priority = 1
	LowPriority    Priority = 2
)

var PriorityMap = map[types.MirrorPriority]Priority{
	types.HighMirrorPriority:   HighPriority,
	types.MediumMirrorPriority: MediumPriority,
	types.LowMirrorPriority:    LowPriority,
}

type MirrorTask struct {
	MirrorID int64
	Priority Priority
	Index    int
}

type MirrorQueue []*MirrorTask

func (mq MirrorQueue) Len() int { return len(mq) }

func (mq MirrorQueue) Less(i, j int) bool { return mq[i].Priority < mq[j].Priority }

func (mq MirrorQueue) Swap(i, j int) {
	mq[i], mq[j] = mq[j], mq[i]
	mq[i].Index, mq[j].Index = i, j
}

func (mq *MirrorQueue) Push(x interface{}) {
	n := len(*mq)
	item := x.(*MirrorTask)
	item.Index = n
	*mq = append(*mq, item)
}

func (mq *MirrorQueue) Pop() interface{} {
	old := *mq
	n := len(old)
	item := old[n-1]
	item.Index = -1
	*mq = old[0 : n-1]
	return item
}

type PriorityQueue struct {
	Queue MirrorQueue
	lock  sync.Mutex
	cond  *sync.Cond
}

var instance *PriorityQueue
var once sync.Once

func NewPriorityQueue() *PriorityQueue {
	mq := &PriorityQueue{
		Queue: MirrorQueue{},
	}
	mq.cond = sync.NewCond(&mq.lock)
	heap.Init(&mq.Queue)
	return mq
}

func (pq *PriorityQueue) Push(mt *MirrorTask) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	heap.Push(&pq.Queue, mt)
	pq.cond.Signal()
}

func (pq *PriorityQueue) Pop() *MirrorTask {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	for pq.Queue.Len() == 0 {
		pq.cond.Wait()
	}
	return heap.Pop(&pq.Queue).(*MirrorTask)
}

func GetPriorityQueueInstance() *PriorityQueue {
	once.Do(func() {
		instance = NewPriorityQueue()
	})
	return instance
}
