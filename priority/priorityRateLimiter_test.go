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
			nl.Wait(pr)
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
	assert.Equal(t, 0, nl.waitList.Len())
}
