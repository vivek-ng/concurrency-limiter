package limiter

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"
)

var ErrTimeout = errors.New("limiter: timed out waiting for capacity")

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

// WithTimeout : If this field is specified , goroutines will be removed from the waitlist
// after the time passes the timeout specified and Wait will return ErrTimeout.
func WithTimeout(timeout int) func(*Limiter) {
	return func(l *Limiter) {
		l.Timeout = &timeout
	}
}

// Wait waits until capacity is available or the context/timeout expires.
// It returns nil only when the caller successfully acquires capacity.
func (l *Limiter) Wait(ctx context.Context) error {
	ok, ch := l.proceed()
	if ok {
		return nil
	}
	if l.Timeout != nil {
		timer := time.NewTimer(time.Duration(*l.Timeout) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ch:
			return nil
		case <-timer.C:
			if l.removeWaiter(ch) {
				return ErrTimeout
			}
			return nil
		case <-ctx.Done():
			if l.removeWaiter(ch) {
				return ctx.Err()
			}
			return nil
		}
	}
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		if l.removeWaiter(ch) {
			return ctx.Err()
		}
		return nil
	}
}

func (l *Limiter) removeWaiter(ch chan struct{}) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for w := l.waitList.Front(); w != nil; w = w.Next() {
		ele := w.Value.(waiter)
		if ele.done == ch {
			close(ch)
			l.waitList.Remove(w)
			return true
		}
	}
	return false
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
	if l.count == 0 {
		return
	}
	l.count -= 1
	first := l.waitList.Front()
	if first == nil {
		return
	}
	w := l.waitList.Remove(first).(waiter)
	l.count++
	close(w.done)
}

// Run wraps the function to limit the concurrency.....
func (l *Limiter) Run(ctx context.Context, callback func() error) error {
	if err := l.Wait(ctx); err != nil {
		return err
	}
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
