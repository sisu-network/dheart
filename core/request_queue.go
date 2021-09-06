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
// and true otherwise. Priorities of different work types are as follow:
//      - 1) keygen
//      - 2) Forced presign that will be used for signing work
//      - 3) Signing
//      - 4) Presign.
func (q *requestQueue) AddWork(work *types.WorkRequest) bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	for _, w := range q.queue {
		if w.WorkId == work.WorkId {
			return false
		}
	}

	priority := work.GetPriority()
	// Insert the work into appropriate position of the queue
	var position int
	for position = len(q.queue) - 1; position >= 0; position-- {
		if priority <= q.queue[position].GetPriority() {
			break
		}
	}

	if position == -1 {
		// Add this to the front of the queue
		q.queue = append([]*types.WorkRequest{work}, q.queue...)
	} else {
		// Insert into the middle of the queue
		first := q.queue[:position]
		second := q.queue[position+1:]

		q.queue = append(first, work)
		q.queue = append(q.queue, second...)
	}

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
