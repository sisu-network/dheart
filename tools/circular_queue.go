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
		lock:  &sync.RWMutex{},
	}
}

func (q *defaultCircularQueue) Add(key string, T any) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.queue[q.tail%q.size] = &wrappedItem{
		key:  key,
		item: T,
	}
	q.tail++

	if q.tail-q.head > q.size {
		q.head++
	}
}

func (q *defaultCircularQueue) Get(key string) (T any) {
	q.lock.RLock()
	defer q.lock.RUnlock()

	// for simplicity, just use a for loop to run through the queue
	for i := q.tail - 1; i >= q.head; i-- {
		index := i % q.size
		if q.queue[index].key == key {
			return q.queue[index].item
		}
	}

	return nil
}
