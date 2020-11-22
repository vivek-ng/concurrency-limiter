package queue

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriorityQueue(t *testing.T) {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	for i := 0; i < 3; i++ {
		item := &Item{
			Priority: i * 2,
			Done:     make(chan struct{}),
		}
		heap.Push(&pq, item)
	}
	expectedVals := []int{4, 2, 0}
	actualVals := make([]int, 0)
	for pq.Len() > 0 {
		topEle := pq.Top().(Item)
		_ = heap.Pop(&pq).(*Item)
		actualVals = append(actualVals, topEle.Priority)
	}
	assert.Equal(t, expectedVals, actualVals)
}

func TestPriorityQueue_SamePriority(t *testing.T) {
	pq := make(PriorityQueue, 3)

	for i := 0; i < 3; i++ {
		pq[i] = &Item{
			Priority:  1,
			timeStamp: int64(i),
		}
	}

	pq.Update(pq[2], 3)
	expectedVals := []int64{2, 0, 1}
	actualVals := make([]int64, 0)
	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*Item)
		actualVals = append(actualVals, item.timeStamp)
	}
	assert.Equal(t, expectedVals, actualVals)
}
