package tools

import (
	"sync"
)

type CircularQueue interface {
	Add(key string, T any)
	Get(key string) any
}

type wrappedItem struct {
	key  string
	item any
}

type defaultCircularQueue struct {
	head, tail int64
	size       int64
	queue      []*wrappedItem
	m          map[string]*wrappedItem // map for quicker look up

	lock *sync.RWMutex
}

func NewCircularQueue(size int) CircularQueue {
	if size <= 0 {
		return nil
	}

	return &defaultCircularQueue{
		head:  0,
		tail:  0,
		size:  int64(size),
		queue: make([]*wrappedItem, size),
		m:     make(map[string]*wrappedItem),
		lock:  &sync.RWMutex{},
	}
}

func (q *defaultCircularQueue) Add(key string, T any) {
	q.lock.Lock()
	defer q.lock.Unlock()

	// Remove old item from the map. The item in the queue will be overwritten.
	if q.tail-q.head >= q.size {
		delete(q.m, q.queue[q.head%q.size].key)
		q.head++
	}

	entry := &wrappedItem{
		key:  key,
		item: T,
	}
	q.queue[q.tail%q.size] = entry
	q.tail++
	q.m[key] = entry
}

func (q *defaultCircularQueue) Get(key string) (T any) {
	q.lock.RLock()
	defer q.lock.RUnlock()

	entry := q.m[key]
	if entry == nil {
		return nil
	}

	return entry.item
}
