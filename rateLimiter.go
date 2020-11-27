package limiter

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// waiter is the individual goroutine waiting for accessing the resource.
// waiter waits for the signal through the done channel.
type waiter struct {
	done chan struct{}
}

// Limiter stores the configuration need for concurrency limiter....
type Limiter struct {
	count    int
	Limit    int
	mu       sync.Mutex
	waitList list.List
	Timeout  *int
}

// Option is a type to configure the Limiter struct....
type Option func(*Limiter)

// New creates an instance of *Limiter. Configure the Limiter with the options specified.
// Example: limiter.New(4, WithTimeout(5))
func New(limit int, options ...Option) *Limiter {
	l := &Limiter{
		Limit: limit,
	}

	for _, o := range options {
		o(l)
	}
	return l
}

// WithTimeout : If this field is specified , goroutines will be automatically removed from the waitlist
// after the time passes the timeout specified even if the number of concurrent requests is greater than the limit.
func WithTimeout(timeout int) func(*Limiter) {
	return func(l *Limiter) {
		l.Timeout = &timeout
	}
}

// Wait method waits if the number of concurrent requests is more than the limit specified.
// If a timeout is configured , then the goroutine will wait until the timeout occurs and then proceeds to
// access the resource irrespective of whether it has received a signal in the done channel.
func (l *Limiter) Wait(ctx context.Context) {
	ok, ch := l.proceed()
	if ok {
		return
	}
	if l.Timeout != nil {
		select {
		case <-ch:
		case <-time.After((time.Duration(*l.Timeout) * time.Millisecond)):
			l.removeWaiter(ch)
		case <-ctx.Done():
		}
		return
	}
	select {
	case <-ch:
	case <-ctx.Done():
		l.removeWaiter(ch)
	}
}

func (l *Limiter) removeWaiter(ch chan struct{}) {
	l.mu.Lock()
	for w := l.waitList.Front(); w != nil; w = w.Next() {
		ele := w.Value.(waiter)
		if ele.done == ch {
			close(ch)
			l.count++
			l.waitList.Remove(w)
			break
		}
	}
	l.mu.Unlock()
}

// proceed will return true if the number of concurrent requests is less than the limit else it
// will add the goroutine to the waiting list and will return a channel. This channel is used by goutines to
// check for signal when they are granted access to use the resource.
func (l *Limiter) proceed() (bool, chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.count < l.Limit {
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

// Finish will remove the goroutine from the waiting list and sends a signal
// to the waiting goroutine to access the resource
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
	l.count++
	close(w.done)
}

// Execute wraps the function to limit the concurrency.....
func (l *Limiter) Run(ctx context.Context, callback func() error) error {
	l.Wait(ctx)
	defer l.Finish()
	return callback()
}

// only used in tests
func (l *Limiter) waitListSize() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	len := l.waitList.Len()
	return len
}

// Count returns the current number of concurrent gouroutines executing...
func (l *Limiter) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}
