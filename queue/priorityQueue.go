package queue

import (
	"container/heap"
	"time"
)

// Item stores the attributes which will be pushed to the priority queue..
type Item struct {
	Done      chan struct{}
	Priority  int
	timeStamp int64
	index     int
}

// PriorityQueue ....
type PriorityQueue []*Item

// Len: Size of the priority queue . Used to satisfy the heap interface...
func (pq PriorityQueue) Len() int { return len(pq) }

// Less is used to compare elements and store them in the proper order in
// priority queue.
func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].Priority == pq[j].Priority {
		return pq[i].timeStamp <= pq[j].timeStamp
	}
	return pq[i].Priority > pq[j].Priority
}

// Swap is used to swap the values in the priority queue.
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push adds elements to the priority queue
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	item.timeStamp = makeTimestamp()
	*pq = append(*pq, item)
}

// Pop removes elements from the priority queue
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// Top returns the topmost element in the priority queue
func (pq *PriorityQueue) Top() interface{} {
	n := len(*pq)
	if n == 0 {
		return nil
	}
	ol := *pq
	return *ol[0]
}

// GetIndex returns the index of the corresponding element.
func (pq *PriorityQueue) GetIndex(x interface{}) int {
	item := x.(*Item)
	return item.index
}

// Update updates the attributes of an element in the priority queue.
func (pq *PriorityQueue) Update(item *Item, priority int) {
	item.Priority = priority
	heap.Fix(pq, item.index)
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
