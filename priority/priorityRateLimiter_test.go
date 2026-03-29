package priority

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	limiter "github.com/vivek-ng/concurrency-limiter"
	"github.com/stretchr/testify/assert"
	"github.com/vivek-ng/concurrency-limiter/queue"
)

func TestPriorityLimiterFinishReleasesHighestPriorityWaiter(t *testing.T) {
	nl := NewLimiter(1)
	ok, _ := nl.proceed(Low)
	assert.True(t, ok)

	ok, low := nl.proceed(Low)
	assert.False(t, ok)
	ok, high := nl.proceed(High)
	assert.False(t, ok)

	nl.Finish()

	select {
	case <-high.Done:
	default:
		t.Fatal("expected high priority waiter to be released first")
	}

	select {
	case <-low.Done:
		t.Fatal("did not expect low priority waiter to be released first")
	default:
	}

	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	select {
	case <-low.Done:
	default:
		t.Fatal("expected low priority waiter to be released second")
	}
	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityLimiterTimeout(t *testing.T) {
	nl := NewLimiter(1, WithTimeout(100))
	assert.NoError(t, nl.Wait(context.Background(), Low))

	var wg sync.WaitGroup
	results := make(chan error, 4)
	ctx := context.Background()

	wg.Add(4)
	for i := 0; i < 4; i++ {
		go func() {
			defer wg.Done()
			results <- nl.Wait(ctx, Low)
		}()
	}

	wg.Wait()
	close(results)

	var timeout int
	for err := range results {
		if !errors.Is(err, limiter.ErrTimeout) {
			t.Fatalf("unexpected error: %v", err)
		}
		timeout++
	}

	assert.Equal(t, 4, timeout)
	assert.Zero(t, nl.waitListSize())
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityLimiterContextDone(t *testing.T) {
	nl := NewLimiter(1)
	assert.NoError(t, nl.Wait(context.Background(), Low))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	results := make(chan error, 3)

	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			results <- nl.Wait(ctx, Low)
		}()
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	wg.Wait()
	close(results)

	var canceled int
	for err := range results {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: %v", err)
		}
		canceled++
	}

	assert.Equal(t, 3, canceled)
	assert.Zero(t, nl.waitListSize())
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestDynamicPriorityPromotesLowPriorityWaiters(t *testing.T) {
	nl := NewLimiter(1, WithDynamicPriority(10))
	assert.NoError(t, nl.Wait(context.Background(), High))

	var wg sync.WaitGroup
	results := make(chan error, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			results <- nl.Wait(context.Background(), Low)
		}()
	}

	time.Sleep(120 * time.Millisecond)
	assert.Equal(t, 2, nl.waitListSize())

	top := nl.waitList.Top().(queue.Item)
	assert.GreaterOrEqual(t, top.Priority, int(High))

	nl.Finish()
	nl.Finish()
	nl.Finish()
	wg.Wait()
	close(results)
	for err := range results {
		assert.NoError(t, err)
	}
	assert.Zero(t, nl.Count())
}

func TestDynamicPriorityCanceledWaiterIsRemovedSafely(t *testing.T) {
	nl := NewLimiter(1, WithDynamicPriority(5))
	assert.NoError(t, nl.Wait(context.Background(), High))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- nl.Wait(ctx, Low)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	assert.ErrorIs(t, <-done, context.Canceled)
	assert.Zero(t, nl.waitListSize())
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestRunDoesNotExecuteOnTimeout(t *testing.T) {
	nl := NewLimiter(1, WithTimeout(50))
	assert.NoError(t, nl.Wait(context.Background(), High))

	var called int32
	err := nl.Run(context.Background(), Low, func() error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	assert.ErrorIs(t, err, limiter.ErrTimeout)
	assert.Zero(t, atomic.LoadInt32(&called))
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityQueueOrderingStillMatchesLimiterExpectations(t *testing.T) {
	nl := NewLimiter(0)
	_, _ = nl.proceed(Low)
	_, _ = nl.proceed(High)
	_, _ = nl.proceed(Medium)

	first := heap.Pop(&nl.waitList).(*queue.Item)
	second := heap.Pop(&nl.waitList).(*queue.Item)
	third := heap.Pop(&nl.waitList).(*queue.Item)

	assert.Equal(t, int(High), first.Priority)
	assert.Equal(t, int(Medium), second.Priority)
	assert.Equal(t, int(Low), third.Priority)
}
