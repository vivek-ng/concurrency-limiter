package priority

import (
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

	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(PriorityValue(pr))
		}(i)
	}
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 2, nl.waitList.Len())
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
	assert.Zero(t, nl.waitList.Len())
}

func TestDynamicPriority(t *testing.T) {
	nl := NewLimiter(3).WithDynamicPriority(10)
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(1)
		}(i)
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, nl.waitList.Len())
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
	nl := NewLimiter(3).WithTimeout(100)
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(pr int) {
			defer wg.Done()
			nl.Wait(1)
		}(i)
	}

	time.Sleep(200 * time.Millisecond)
	wg.Wait()
	for i := 0; i < 5; i++ {
		nl.Finish()
	}
	assert.Zero(t, nl.count)
	assert.Zero(t, nl.waitList.Len())
}
