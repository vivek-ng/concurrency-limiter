package priority

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vivek-ng/concurrency-limiter/queue"
)

func TestPriorityLimiter(t *testing.T) {
	nl := NewLimiter(3)
	var wg sync.WaitGroup
	wg.Add(5)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(ctx, PriorityValue(pr))
		}(i)
	}
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 2, nl.waitListSize())
	pVal := nl.waitList.Top()
	pValItem := pVal.(queue.Item)
	expectedVal1 := pValItem.Priority
	nl.Finish()
	pVal = nl.waitList.Top()
	pValItem = pVal.(queue.Item)
	expectedVal2 := pValItem.Priority
	assert.Greater(t, expectedVal1, expectedVal2)
	nl.Finish()
	wg.Wait()
	assert.Zero(t, nl.waitListSize())
}

func TestDynamicPriority(t *testing.T) {
	nl := NewLimiter(3,
		WithDynamicPriority(10),
	)
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(ctx, 1)
		}(i)
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, nl.waitListSize())
	pVal := nl.waitList.Top()
	pValItem := pVal.(queue.Item)
	expectedVal1 := pValItem.Priority
	nl.Finish()
	pVal = nl.waitList.Top()
	pValItem = pVal.(queue.Item)
	expectedVal2 := pValItem.Priority
	assert.GreaterOrEqual(t, expectedVal1, int(High))
	assert.GreaterOrEqual(t, expectedVal2, int(High))
}

func TestPriorityLimiter_Timeout(t *testing.T) {
	nl := NewLimiter(3,
		WithTimeout(100),
	)
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(ctx, 1)
		}(i)
	}

	time.Sleep(200 * time.Millisecond)
	wg.Wait()
	for i := 0; i < 5; i++ {
		nl.Finish()
	}
	assert.Zero(t, nl.count)
	assert.Zero(t, nl.waitListSize())
}

func TestPriorityLimiter_ContextDone(t *testing.T) {
	nl := NewLimiter(3)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(ctx, 1)
		}(i)
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, nl.waitListSize())
	cancel()
	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, nl.waitListSize())
}

func TestPriorityLimiter_ContextWithTimeout(t *testing.T) {
	nl := NewLimiter(3,
		WithTimeout(500))
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(ctx, 1)
		}(i)
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, nl.waitListSize())
	cancel()
	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, nl.waitListSize())
}

func TestDynamicPriorityAndTimeout(t *testing.T) {
	nl := NewLimiter(3,
		WithTimeout(100),
		WithDynamicPriority(5))
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(ctx, 1)
		}(i)
	}
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, nl.waitListSize())
}

func TestDynamicPriorityWithTimeout_ContextDone(t *testing.T) {
	nl := NewLimiter(3,
		WithTimeout(300),
		WithDynamicPriority(5))
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(ctx, 1)
		}(i)
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, nl.waitListSize())
}

func TestExecute(t *testing.T) {
	l := NewLimiter(2)
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			_ = l.Execute(ctx, Low, func() error {
				return nil
			})
		}()
	}

	wg.Wait()
	assert.Zero(t, l.waitListSize())
	assert.Zero(t, l.count)
}
