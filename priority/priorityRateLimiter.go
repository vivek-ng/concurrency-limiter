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

func (p *PriorityLimiter) Wait(priority int) {
	ok, ch := p.proceed(priority)
	if !ok {
		<-ch
	}
}

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
