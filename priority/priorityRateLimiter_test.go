package priority

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	limiter "github.com/vivek-ng/concurrency-limiter"
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

	assert.True(t, errors.Is(<-done, context.Canceled))
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

	assert.True(t, errors.Is(err, limiter.ErrTimeout))
	assert.Zero(t, atomic.LoadInt32(&called))
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityLimiterWaitOrBypassReturnsBypassedOnTimeout(t *testing.T) {
	nl := NewLimiter(1, WithTimeout(50))
	assert.NoError(t, nl.Wait(context.Background(), High))

	result, err := nl.WaitOrBypass(context.Background(), Low)

	assert.NoError(t, err)
	assert.Equal(t, limiter.AdmissionBypassed, result)
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityLimiterWaitOrBypassReturnsCanceledContext(t *testing.T) {
	nl := NewLimiter(1, WithTimeout(100))
	assert.NoError(t, nl.Wait(context.Background(), High))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := nl.WaitOrBypass(ctx, Low)

	assert.Zero(t, result)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityRunOrBypassExecutesCallbackOnTimeoutWithoutFinishing(t *testing.T) {
	nl := NewLimiter(1, WithTimeout(50))
	assert.NoError(t, nl.Wait(context.Background(), High))

	var called int32
	result, err := nl.RunOrBypass(context.Background(), Low, func() error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, limiter.AdmissionBypassed, result)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityRunOrBypassAcquiredStillFinishes(t *testing.T) {
	nl := NewLimiter(1, WithTimeout(50))

	var called int32
	result, err := nl.RunOrBypass(context.Background(), Low, func() error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, limiter.AdmissionAcquired, result)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
	assert.Zero(t, nl.Count())
}

func TestPrioritySoftModeStillReleasesHighestPriorityBeforeBypassing(t *testing.T) {
	nl := NewLimiter(1, WithTimeout(100))
	assert.NoError(t, nl.Wait(context.Background(), Low))

	highDone := make(chan limiter.AdmissionResult, 1)
	lowDone := make(chan limiter.AdmissionResult, 1)

	go func() {
		result, err := nl.WaitOrBypass(context.Background(), High)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		highDone <- result
	}()
	go func() {
		result, err := nl.WaitOrBypass(context.Background(), Low)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		lowDone <- result
	}()

	time.Sleep(30 * time.Millisecond)
	nl.Finish()

	assert.Equal(t, limiter.AdmissionAcquired, <-highDone)
	assert.Equal(t, 1, nl.Count())

	nl.Finish()

	assert.Equal(t, limiter.AdmissionAcquired, <-lowDone)
	assert.Equal(t, 1, nl.Count())

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityExportedFieldMutationDoesNotAffectRuntimeLimit(t *testing.T) {
	nl := NewLimiter(1)
	nl.Limit = 100

	assert.NoError(t, nl.Wait(context.Background(), High))
	assert.Equal(t, 1, nl.Count())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- nl.Wait(ctx, Low)
	}()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, nl.waitListSize())
	cancel()
	assert.True(t, errors.Is(<-done, context.Canceled))

	nl.Finish()
	assert.Zero(t, nl.Count())
}

func TestPriorityExportedFieldMutationDoesNotAffectRuntimeTimeouts(t *testing.T) {
	nl := NewLimiter(1, WithTimeout(50), WithDynamicPriority(5))
	nl.Timeout = nil
	nl.DynamicPeriod = nil

	assert.NoError(t, nl.Wait(context.Background(), High))

	done := make(chan error, 1)
	go func() {
		done <- nl.Wait(context.Background(), Low)
	}()

	err := <-done
	assert.True(t, errors.Is(err, limiter.ErrTimeout))

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
