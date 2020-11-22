package queue

import (
	"container/heap"
	"time"
)

type Item struct {
	Done      chan struct{}
	Priority  int
	timeStamp int64
	index     int
}

type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].Priority == pq[j].Priority {
		return pq[i].timeStamp <= pq[j].timeStamp
	}
	return pq[i].Priority > pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	item.timeStamp = makeTimestamp()
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) Top() interface{} {
	n := len(*pq)
	if n == 0 {
		return nil
	}
	ol := *pq
	return *ol[0]
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func (pq *PriorityQueue) Update(item *Item, priority int) {
	item.Priority = priority
	heap.Fix(pq, item.index)
}
