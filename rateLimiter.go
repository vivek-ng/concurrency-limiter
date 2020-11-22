package limiter

import (
	"container/list"
	"sync"
	"time"
)

// waiter is the individual goroutine waiting for accessing the request.
// waiter waits for the signal through the done channel.
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

// Wait waits if the number of concurrent requests is more than the limit specified.
// If a timeout is configured , then the goroutine will wait until the timeout occurs and then proceeds to
// access the resource irrespective of whether it has received a signal in the done channel.
func (l *Limiter) Wait() {
	ok, ch := l.proceed()
	if ok {
		return
	}
	if l.timeout != nil {
		select {
		case <-ch:
		case <-time.After((time.Duration(*l.timeout) * time.Millisecond)):
			l.mu.Lock()
			for w := l.waitList.Front(); w != nil; w = w.Next() {
				ele := w.Value.(waiter)
				if ele.done == ch {
					close(ch)
					l.waitList.Remove(w)
					l.count += 1
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
	first := l.waitList.Front()
	if first == nil {
		return
	}
	w := l.waitList.Remove(first).(waiter)
	w.done <- struct{}{}
	close(w.done)
}
