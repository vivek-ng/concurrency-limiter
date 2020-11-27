package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/vivek-ng/concurrency-limiter/priority"
)

func main() {
	pr := priority.NewLimiter(1)
	var wg sync.WaitGroup
	wg.Add(15)
	for i := 0; i < 15; i++ {
		go func(index int) {
			defer wg.Done()
			ctx := context.Background()
			if index%2 == 1 {
				pr.Wait(ctx, priority.High)
			} else {
				pr.Wait(ctx, priority.Low)
			}
			fmt.Println("executing action...: ", "index: ", index, "current number of goroutines: ", pr.Count())
			pr.Finish()
		}(i)
	}
	wg.Wait()
}
