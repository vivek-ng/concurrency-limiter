package limiter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConcurrentRateLimiterNonBlocking(t *testing.T) {
	l := New(7)

	var wg sync.WaitGroup
	results := make(chan error, 5)
	wg.Add(5)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			results <- l.Wait(ctx)
		}()
	}

	wg.Wait()
	close(results)
	for err := range results {
		assert.NoError(t, err)
	}
	assert.Zero(t, l.waitList.Len())
	assert.Equal(t, 5, l.Count())
	for i := 0; i < 5; i++ {
		l.Finish()
	}
	assert.Zero(t, l.Count())
}

func TestConcurrentRateLimiterBlocking(t *testing.T) {
	l := New(2)

	var wg sync.WaitGroup
	results := make(chan error, 5)
	wg.Add(5)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			results <- l.Wait(ctx)
		}()
	}

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 3, l.waitListSize())
	assert.Equal(t, 2, l.Count())

	for i := 0; i < 5; i++ {
		l.Finish()
	}

	wg.Wait()
	close(results)
	for err := range results {
		assert.NoError(t, err)
	}
	assert.Equal(t, 0, l.waitListSize())
	assert.Zero(t, l.Count())
}

func TestConcurrentRateLimiterTimeout(t *testing.T) {
	l := New(2, WithTimeoutDuration(100*time.Millisecond))

	var wg sync.WaitGroup
	results := make(chan error, 5)
	ctx := context.Background()

	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			results <- l.Wait(ctx)
		}()
	}

	wg.Wait()
	close(results)

	var success, timeout int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrTimeout):
			timeout++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	assert.Equal(t, 2, success)
	assert.Equal(t, 3, timeout)
	assert.Equal(t, 2, l.Count())
	assert.Zero(t, l.waitList.Len())

	l.Finish()
	l.Finish()
	assert.Zero(t, l.Count())
}

func TestConcurrentRateLimiterContextDone(t *testing.T) {
	l := New(2)

	var wg sync.WaitGroup
	results := make(chan error, 5)
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			results <- l.Wait(ctx)
		}()
	}

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 3, l.waitListSize())
	cancel()

	wg.Wait()
	close(results)

	var success, canceled int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, context.Canceled):
			canceled++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	assert.Equal(t, 2, success)
	assert.Equal(t, 3, canceled)
	assert.Zero(t, l.waitListSize())
	assert.Equal(t, 2, l.Count())

	l.Finish()
	l.Finish()
	assert.Zero(t, l.Count())
}

func TestRunDoesNotExecuteOnCanceledWait(t *testing.T) {
	l := New(1)
	assert.NoError(t, l.Wait(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var called int32
	err := l.Run(ctx, func() error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	assert.True(t, errors.Is(err, context.Canceled))
	assert.Zero(t, atomic.LoadInt32(&called))
	assert.Equal(t, 1, l.Count())

	l.Finish()
	assert.Zero(t, l.Count())
}

func TestRunDoesNotExecuteOnTimeout(t *testing.T) {
	l := New(1, WithTimeoutDuration(50*time.Millisecond))
	assert.NoError(t, l.Wait(context.Background()))

	var called int32
	err := l.Run(context.Background(), func() error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	assert.True(t, errors.Is(err, ErrTimeout))
	assert.Zero(t, atomic.LoadInt32(&called))
	assert.Equal(t, 1, l.Count())

	l.Finish()
	assert.Zero(t, l.Count())
}

func TestWaitOrBypassReturnsBypassedOnTimeout(t *testing.T) {
	l := New(1, WithTimeoutDuration(50*time.Millisecond))
	assert.NoError(t, l.Wait(context.Background()))

	result, err := l.WaitOrBypass(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, AdmissionBypassed, result)
	assert.Equal(t, 1, l.Count())

	l.Finish()
	assert.Zero(t, l.Count())
}

func TestWaitOrBypassReturnsCanceledContext(t *testing.T) {
	l := New(1, WithTimeoutDuration(100*time.Millisecond))
	assert.NoError(t, l.Wait(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := l.WaitOrBypass(ctx)

	assert.Zero(t, result)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, 1, l.Count())

	l.Finish()
	assert.Zero(t, l.Count())
}

func TestRunOrBypassExecutesCallbackOnTimeoutWithoutFinishing(t *testing.T) {
	l := New(1, WithTimeoutDuration(50*time.Millisecond))
	assert.NoError(t, l.Wait(context.Background()))

	var called int32
	result, err := l.RunOrBypass(context.Background(), func() error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, AdmissionBypassed, result)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
	assert.Equal(t, 1, l.Count())

	l.Finish()
	assert.Zero(t, l.Count())
}

func TestRunOrBypassAcquiredStillFinishes(t *testing.T) {
	l := New(1, WithTimeoutDuration(50*time.Millisecond))

	var called int32
	result, err := l.RunOrBypass(context.Background(), func() error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, AdmissionAcquired, result)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
	assert.Zero(t, l.Count())
}

func TestExportedFieldMutationDoesNotAffectRuntimeLimit(t *testing.T) {
	l := New(1)
	l.Limit = 100

	assert.NoError(t, l.Wait(context.Background()))
	assert.Equal(t, 1, l.Count())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- l.Wait(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, l.waitListSize())
	cancel()
	assert.True(t, errors.Is(<-done, context.Canceled))

	l.Finish()
	assert.Zero(t, l.Count())
}

func TestExportedFieldMutationDoesNotAffectRuntimeTimeout(t *testing.T) {
	l := New(1, WithTimeoutDuration(50*time.Millisecond))
	l.Timeout = nil

	assert.NoError(t, l.Wait(context.Background()))

	done := make(chan error, 1)
	go func() {
		done <- l.Wait(context.Background())
	}()

	err := <-done
	assert.True(t, errors.Is(err, ErrTimeout))

	l.Finish()
	assert.Zero(t, l.Count())
}

func TestDeprecatedWithTimeoutStillUsesMilliseconds(t *testing.T) {
	l := New(1, WithTimeout(50))
	assert.Equal(t, 50, *l.Timeout)

	assert.NoError(t, l.Wait(context.Background()))

	done := make(chan error, 1)
	go func() {
		done <- l.Wait(context.Background())
	}()

	assert.True(t, errors.Is(<-done, ErrTimeout))

	l.Finish()
	assert.Zero(t, l.Count())
}
