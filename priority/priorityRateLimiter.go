package priority

import (
	"container/heap"
	"context"
	"sync"
	"time"

	limiter "github.com/vivek-ng/concurrency-limiter"
	"github.com/vivek-ng/concurrency-limiter/queue"
)

// PriorityValue defines the priority values of goroutines.
// Greater priority value means higher priority
type PriorityValue int

const (
	// Low Priority....
	Low PriorityValue = 1
	// Medium Priority....
	Medium PriorityValue = 2
	// MediumHigh Priority....
	MediumHigh PriorityValue = 3
	// High Priority.....
	High PriorityValue = 4
)

// PriorityLimiter stores the configuration need for priority concurrency limiter....
type PriorityLimiter struct {
	count int
	// Deprecated: configure via NewLimiter. Runtime behavior uses an internal snapshot.
	Limit    int
	mu       sync.Mutex
	waitList queue.PriorityQueue
	// Deprecated: configure via WithDynamicPriorityDuration. Runtime behavior uses an internal snapshot.
	DynamicPeriod *int
	// Deprecated: configure via WithTimeoutDuration. Runtime behavior uses an internal snapshot.
	Timeout *int

	limit         int
	dynamicPeriod *time.Duration
	timeout       *time.Duration
}

// Option is a type to configure the Limiter struct....
type Option func(*PriorityLimiter)

// NewLimiter creates an instance of *PriorityLimiter. Configure the Limiter with the options specified.
// Example: priority.NewLimiter(4, WithDynamicPriorityDuration(5*time.Millisecond))
func NewLimiter(limit int, options ...Option) *PriorityLimiter {
	pq := make(queue.PriorityQueue, 0)
	nl := &PriorityLimiter{
		Limit:    limit,
		waitList: pq,
		limit:    limit,
	}

	for _, o := range options {
		o(nl)
	}

	heap.Init(&pq)
	return nl
}

// WithDynamicPriority configures dynamic priority cadence in milliseconds.
// Deprecated: use WithDynamicPriorityDuration.
func WithDynamicPriority(dynamicPeriod int) func(*PriorityLimiter) {
	return WithDynamicPriorityDuration(time.Duration(dynamicPeriod) * time.Millisecond)
}

// WithDynamicPriorityDuration configures the dynamic priority cadence as a time.Duration.
func WithDynamicPriorityDuration(dynamicPeriod time.Duration) func(*PriorityLimiter) {
	return func(p *PriorityLimiter) {
		ms := int(dynamicPeriod / time.Millisecond)
		p.DynamicPeriod = &ms
		p.dynamicPeriod = &dynamicPeriod
	}
}

// WithTimeout configures timeout in milliseconds.
// Deprecated: use WithTimeoutDuration.
func WithTimeout(timeout int) func(*PriorityLimiter) {
	return WithTimeoutDuration(time.Duration(timeout) * time.Millisecond)
}

// WithTimeoutDuration configures timeout as a time.Duration.
// Goroutines are removed from the waitlist after the timeout and Wait returns limiter.ErrTimeout.
func WithTimeoutDuration(timeout time.Duration) func(*PriorityLimiter) {
	return func(p *PriorityLimiter) {
		ms := int(timeout / time.Millisecond)
		p.Timeout = &ms
		p.timeout = &timeout
	}
}

// Wait method waits if the number of concurrent requests is more than the limit specified.
// If the priority of two goroutines are same , the FIFO order is followed.
// Greater priority value means higher priority.
// priority must be one fo the values specified by PriorityValue
//
// Low = 1
// Medium = 2
// MediumHigh = 3
// High = 4
func (p *PriorityLimiter) Wait(ctx context.Context, priority PriorityValue) error {
	_, err := p.wait(ctx, priority, false)
	return err
}

// WaitOrBypass waits until capacity is available, or bypasses the limiter after the configured timeout.
func (p *PriorityLimiter) WaitOrBypass(ctx context.Context, priority PriorityValue) (limiter.AdmissionResult, error) {
	return p.wait(ctx, priority, true)
}

func (p *PriorityLimiter) wait(ctx context.Context, priority PriorityValue, allowBypass bool) (limiter.AdmissionResult, error) {
	ok, w := p.proceed(priority)
	if ok {
		return limiter.AdmissionAcquired, nil
	}

	if p.dynamicPeriod == nil && p.timeout == nil {
		select {
		case <-w.Done:
			return limiter.AdmissionAcquired, nil
		case <-ctx.Done():
			if p.removeWaiter(w) {
				return 0, ctx.Err()
			}
			return limiter.AdmissionAcquired, nil
		}
	}

	if p.dynamicPeriod != nil && p.timeout != nil {
		return p.dynamicPriorityAndTimeout(ctx, w, allowBypass)
	}

	if p.timeout != nil {
		return p.handleTimeout(ctx, w, allowBypass)
	}

	return p.handleDynamicPriority(ctx, w)
}

func (p *PriorityLimiter) dynamicPriorityAndTimeout(ctx context.Context, w *queue.Item, allowBypass bool) (limiter.AdmissionResult, error) {
	ticker := time.NewTicker(*p.dynamicPeriod)
	timer := time.NewTimer(*p.timeout)
	defer ticker.Stop()
	defer timer.Stop()
	for {
		select {
		case <-w.Done:
			return limiter.AdmissionAcquired, nil
		case <-ctx.Done():
			if p.removeWaiter(w) {
				return 0, ctx.Err()
			}
			return limiter.AdmissionAcquired, nil
		case <-timer.C:
			if p.removeWaiter(w) {
				if allowBypass {
					return limiter.AdmissionBypassed, nil
				}
				return 0, limiter.ErrTimeout
			}
			return limiter.AdmissionAcquired, nil
		case <-ticker.C:
			// edge case where we receive ctx.Done and ticker.C at the same time...
			select {
			case <-ctx.Done():
				if p.removeWaiter(w) {
					return 0, ctx.Err()
				}
				return limiter.AdmissionAcquired, nil
			default:
			}
			p.mu.Lock()
			if w.Priority < int(High) {
				if _, ok := p.waitList.FindIndex(w); !ok {
					p.mu.Unlock()
					return limiter.AdmissionAcquired, nil
				}
				currentPriority := w.Priority
				p.waitList.Update(w, currentPriority+1)
			}
			p.mu.Unlock()
		}
	}
}

func (p *PriorityLimiter) handleDynamicPriority(ctx context.Context, w *queue.Item) (limiter.AdmissionResult, error) {
	ticker := time.NewTicker(*p.dynamicPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-w.Done:
			return limiter.AdmissionAcquired, nil
		case <-ticker.C:
			p.mu.Lock()
			if w.Priority < int(High) {
				if _, ok := p.waitList.FindIndex(w); !ok {
					p.mu.Unlock()
					return limiter.AdmissionAcquired, nil
				}
				currentPriority := w.Priority
				p.waitList.Update(w, currentPriority+1)
			}
			p.mu.Unlock()
		case <-ctx.Done():
			if p.removeWaiter(w) {
				return 0, ctx.Err()
			}
			return limiter.AdmissionAcquired, nil
		}
	}
}

func (p *PriorityLimiter) handleTimeout(ctx context.Context, w *queue.Item, allowBypass bool) (limiter.AdmissionResult, error) {
	timer := time.NewTimer(*p.timeout)
	defer timer.Stop()
	select {
	case <-w.Done:
		return limiter.AdmissionAcquired, nil
	case <-timer.C:
		if p.removeWaiter(w) {
			if allowBypass {
				return limiter.AdmissionBypassed, nil
			}
			return 0, limiter.ErrTimeout
		}
		return limiter.AdmissionAcquired, nil
	case <-ctx.Done():
		if p.removeWaiter(w) {
			return 0, ctx.Err()
		}
		return limiter.AdmissionAcquired, nil
	}
}

func (p *PriorityLimiter) removeWaiter(w *queue.Item) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx, ok := p.waitList.FindIndex(w); ok {
		heap.Remove(&p.waitList, idx)
		close(w.Done)
		return true
	}
	return false
}

// proceed will return true if the number of concurrent requests is less than the limit else it
// will add the goroutine to the priority queue and will return a channel. This channel is used by goutines to
// check for signal when they are granted access to use the resource.
func (p *PriorityLimiter) proceed(priority PriorityValue) (bool, *queue.Item) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.count < p.limit {
		p.count++
		return true, nil
	}
	ch := make(chan struct{})
	w := &queue.Item{
		Priority: int(priority),
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
	if p.count == 0 {
		return
	}
	p.count -= 1
	if p.waitList.Len() == 0 {
		return
	}
	ele := heap.Pop(&p.waitList)
	it := ele.(*queue.Item)
	p.count++
	close(it.Done)
}

// Run wraps the function to limit the concurrency.....
func (p *PriorityLimiter) Run(ctx context.Context,
	priority PriorityValue,
	callback func() error) error {
	if err := p.Wait(ctx, priority); err != nil {
		return err
	}
	defer p.Finish()
	return callback()
}

// RunOrBypass executes the callback after real acquisition or timeout bypass.
// Finish is only called when capacity was actually acquired.
func (p *PriorityLimiter) RunOrBypass(ctx context.Context,
	priority PriorityValue,
	callback func() error) (limiter.AdmissionResult, error) {
	result, err := p.WaitOrBypass(ctx, priority)
	if err != nil {
		return 0, err
	}
	if result == limiter.AdmissionAcquired {
		defer p.Finish()
	}
	return result, callback()
}

// only used in tests
func (p *PriorityLimiter) waitListSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	len := p.waitList.Len()
	return len
}

// Count returns the current number of concurrent gouroutines executing...
func (p *PriorityLimiter) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}
