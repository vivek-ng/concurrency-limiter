package limiter

import (
	"container/list"
	"sync"
)

type waiter struct {
	done chan struct{}
}

type Limiter struct {
	count    int
	limit    int
	mu       sync.Mutex
	waitList list.List
	//notify []chan struct{}
}

func NewLimiter(limit int) *Limiter {
	return &Limiter{
		limit: limit,
	}
}

func (l *Limiter) Wait() {
	ok, ch := l.proceed()
	if !ok {
		<-ch
	}
}

func (l *Limiter) proceed() (bool, chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.count < l.limit {
		l.count++
		return true, nil
	}
	ch := make(chan struct{})
	w := waiter{
		done: ch,
	}
	l.waitList.PushBack(w)
	return false, ch
}

func (l *Limiter) Finish() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count -= 1
	first := l.waitList.Front()
	w := l.waitList.Remove(first).(waiter)
	w.done <- struct{}{}
	close(w.done)
}
