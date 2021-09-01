package core

import (
	"sync"

	"github.com/sisu-network/dheart/worker/types"
)

// TODO: add job priority for this queue. Keygen &signing should have more priority than presign even
// though they could be inserted later.
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

// AddWork adds a work in the queue. It returns false if the work has already existed in the queue
// and true otherwise.
func (q *requestQueue) AddWork(work *types.WorkRequest) bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	for _, w := range q.queue {
		if w.WorkId == work.WorkId {
			return false
		}
	}

	q.queue = append(q.queue, work)
	return true
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
