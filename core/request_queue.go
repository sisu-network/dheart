package core

import (
	"sync"

	"github.com/sisu-network/dheart/worker/types"
)

type requestQueue struct {
	queue []*types.WorkRequest
	lock  *sync.RWMutex
}

func NewRequestQueue() *requestQueue {
	return &requestQueue{
		queue: make([]*types.WorkRequest, 0),
		lock:  &sync.RWMutex{},
	}
}

func (q *requestQueue) AddWork(work *types.WorkRequest) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.queue = append(q.queue, work)
}

func (q *requestQueue) Pop() *types.WorkRequest {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.queue) == 0 {
		return nil
	}

	work := q.queue[0]
	q.queue = q.queue[1:]

	return work
}

func (q *requestQueue) Size() int {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return len(q.queue)
}
