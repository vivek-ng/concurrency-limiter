package limiter

import (
	"container/list"
	"sync"
	"time"
)

type waiter struct {
	done chan struct{}
}

type Limiter struct {
	count    int
	limit    int
	mu       sync.Mutex
	waitList list.List
	timeout  *int
}

func NewLimiter(limit int) *Limiter {
	return &Limiter{
		limit: limit,
	}
}

func (l *Limiter) WithTimeout(timeout int) *Limiter {
	l.timeout = &timeout
	return l
}

func (l *Limiter) Wait() {
	ok, ch := l.proceed()
	if ok {
		return
	}
	if l.timeout != nil {
		select {
		case <-ch:
		case <-time.After((time.Duration(*l.timeout) * time.Second)):
			l.mu.Lock()
			for w := l.waitList.Front(); w != nil; w = w.Next() {
				ele := w.Value.(waiter)
				if ele.done == ch {
					close(ch)
					l.waitList.Remove(w)
					break
				}
			}
			l.mu.Unlock()
		}
		return
	}
	<-ch
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
	l.count = max(0, l.count)
	first := l.waitList.Front()
	if first == nil {
		return
	}
	w := l.waitList.Remove(first).(waiter)
	w.done <- struct{}{}
	close(w.done)
}

func max(a, b int) int {
	if a >= b {
		return a
	}
	return b
}
