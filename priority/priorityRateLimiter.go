package priority

import (
	"container/heap"
	"sync"

	"github.com/vivek-ng/concurrency-limiter/queue"
)

type PriorityLimiter struct {
	count    int
	limit    int
	mu       sync.Mutex
	waitList queue.PriorityQueue
}

func NewLimiter(limit int) *PriorityLimiter {
	pq := make(queue.PriorityQueue, 0)
	nl := &PriorityLimiter{
		limit:    limit,
		waitList: pq,
	}

	heap.Init(&pq)
	return nl
}

// Wait method waits if the number of concurrent requests is more than the limit specified.
// If the priority of two goroutines are same , the FIFO order is followed.
// Greater priority value means higher priority.
func (p *PriorityLimiter) Wait(priority int) {
	ok, ch := p.proceed(priority)
	if !ok {
		<-ch
	}
}

// proceed will return true if the number of concurrent requests is less than the limit else it
// will add the goroutine to the priority queue and will return a channel. This channel is used by goutines to
// check for signal when they are granted access to use the resource.
func (p *PriorityLimiter) proceed(priority int) (bool, chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.count < p.limit {
		p.count++
		return true, nil
	}
	ch := make(chan struct{})
	w := &queue.Item{
		Priority: priority,
		Done:     ch,
	}
	heap.Push(&p.waitList, w)
	return false, ch
}

// Finish will remove the goroutine from the priority queue and sends a signal
// to the waiting goroutine to access the resource
func (p *PriorityLimiter) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.count -= 1
	ele := heap.Pop(&p.waitList)
	if ele == nil {
		return
	}
	it := ele.(*queue.Item)
	it.Done <- struct{}{}
	close(it.Done)
}
