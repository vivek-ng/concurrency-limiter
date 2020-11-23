package priority

import (
	"container/heap"
	"sync"
	"time"

	"github.com/vivek-ng/concurrency-limiter/queue"
)

type PriorityLimiter struct {
	count         int
	limit         int
	mu            sync.Mutex
	waitList      queue.PriorityQueue
	dynamicPeriod *int
	timeout       *int
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

func (p *PriorityLimiter) WithDynamicPriority(dynamicPeriod int) *PriorityLimiter {
	p.dynamicPeriod = &dynamicPeriod
	return p
}

func (p *PriorityLimiter) WithTimeout(timeout int) *PriorityLimiter {
	p.timeout = &timeout
	return p
}

// Wait method waits if the number of concurrent requests is more than the limit specified.
// If the priority of two goroutines are same , the FIFO order is followed.
// Greater priority value means higher priority.
func (p *PriorityLimiter) Wait(priority int) {
	ok, w := p.proceed(priority)
	if ok {
		return
	}

	if p.dynamicPeriod == nil && p.timeout == nil {
		<-w.Done
		return
	}
	if p.dynamicPeriod != nil {
		ticker := time.NewTicker(time.Duration(*p.dynamicPeriod) * time.Millisecond)
		for {
			select {
			case <-w.Done:
				return
			case <-ticker.C:
				p.mu.Lock()
				currentPriority := w.Priority
				p.waitList.Update(w, currentPriority+1)
				p.mu.Unlock()
			}
		}
	}

	select {
	case <-w.Done:
	case <-time.After(time.Duration(*p.timeout) * time.Millisecond):
		p.mu.Lock()
		heap.Remove(&p.waitList, p.waitList.GetIndex(w))
		p.count += 1
		p.mu.Unlock()
	}
}

// proceed will return true if the number of concurrent requests is less than the limit else it
// will add the goroutine to the priority queue and will return a channel. This channel is used by goutines to
// check for signal when they are granted access to use the resource.
func (p *PriorityLimiter) proceed(priority int) (bool, *queue.Item) {
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
	return false, w
}

// Finish will remove the goroutine from the priority queue and sends a signal
// to the waiting goroutine to access the resource
func (p *PriorityLimiter) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.count -= 1
	if p.waitList.Len() == 0 {
		return
	}
	ele := heap.Pop(&p.waitList)
	it := ele.(*queue.Item)
	it.Done <- struct{}{}
	close(it.Done)
}
