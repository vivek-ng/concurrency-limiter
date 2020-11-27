package main

import (
	"context"
	"fmt"
	"sync"

	limiter "github.com/vivek-ng/concurrency-limiter"
)

func main() {
	nl := limiter.New(3)

	var wg sync.WaitGroup
	wg.Add(15)

	for i := 0; i < 15; i++ {
		go func(index int) {
			defer wg.Done()
			ctx := context.Background()
			nl.Wait(ctx)
			fmt.Println("executing action...: ", "index: ", index, "current number of goroutines: ", nl.Count())
			nl.Finish()
		}(i)
	}

	wg.Wait()
}
