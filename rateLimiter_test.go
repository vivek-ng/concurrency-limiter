package limiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConcurrentRateLimiterNonBlocking(t *testing.T) {
	l := New(7)

	var wg sync.WaitGroup
	wg.Add(5)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			l.Wait(ctx)
		}()
	}

	wg.Wait()
	assert.Equal(t, 0, l.waitList.Len())
}

func TestConcurrentRateLimiterBlocking(t *testing.T) {
	l := New(2)

	var wg sync.WaitGroup
	wg.Add(5)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			l.Wait(ctx)
		}()
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 3, l.waitListSize())
	for i := 0; i < 3; i++ {
		l.Finish()
	}
	wg.Wait()
	assert.Equal(t, 0, l.waitListSize())
}

func TestConcurrentRateLimiterTimeout(t *testing.T) {
	l := New(2,
		WithTimeout(300),
	)

	var wg sync.WaitGroup
	wg.Add(5)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			l.Wait(ctx)
		}()
	}
	time.Sleep(500 * time.Millisecond)
	wg.Wait()
	l.Finish()
	l.Finish()
	assert.Equal(t, 3, l.Count())
	assert.Equal(t, 0, l.waitList.Len())
}

func TestConcurrentRateLimiter_ContextDone(t *testing.T) {
	l := New(2)

	var wg sync.WaitGroup
	wg.Add(5)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			l.Wait(ctx)
		}()
	}
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 3, l.waitListSize())
	cancel()
	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, l.waitListSize())
	assert.Equal(t, 5, l.Count())
}

func TestExecute(t *testing.T) {
	l := New(2)
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			_ = l.Execute(ctx, func() error {
				return nil
			})
		}()
	}

	wg.Wait()
	assert.Zero(t, l.waitListSize())
	assert.Zero(t, l.Count())
}
