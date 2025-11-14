package worker

import (
	"context"
	"sync"
)

type Queue struct {
	ch        chan uint // FileIndex IDs
	mu        sync.Mutex
	enqueued  map[uint]struct{}
	accepting bool
}

func NewQueue(buf int) *Queue {
	return &Queue{
		ch:        make(chan uint, buf*2+10),
		enqueued:  make(map[uint]struct{}),
		accepting: true,
	}
}

func (q *Queue) Enqueue(id uint) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.accepting {
		return false
	}
	if _, ok := q.enqueued[id]; ok {
		return false
	}
	q.enqueued[id] = struct{}{}
	q.ch <- id
	return true
}

func (q *Queue) Dequeued(id uint) {
	q.mu.Lock()
	delete(q.enqueued, id)
	q.mu.Unlock()
}

func (q *Queue) StopAccepting() {
	q.mu.Lock()
	q.accepting = false
	q.mu.Unlock()
}

func (q *Queue) Chan() <-chan uint { return q.ch }

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.enqueued)
}

func (q *Queue) Drain(ctx context.Context) {
	// nothing to do explicitly; pool will finish
}
