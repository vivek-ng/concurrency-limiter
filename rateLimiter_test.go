package limiter

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConcurrentRateLimiterNonBlocking(t *testing.T) {
	l := NewLimiter(7)

	var wg sync.WaitGroup
	wg.Add(5)

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			l.Wait()
		}()
	}

	wg.Wait()
	assert.Equal(t, 0, l.waitList.Len())
}

func TestConcurrentRateLimiterBlocking(t *testing.T) {
	l := NewLimiter(2)

	var wg sync.WaitGroup
	wg.Add(5)

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			l.Wait()
		}()
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 3, l.waitList.Len())
	for i := 0; i < 3; i++ {
		l.Finish()
	}
	wg.Wait()
	assert.Equal(t, 0, l.waitList.Len())
}

func TestConcurrentRateLimiterTimeout(t *testing.T) {
	l := NewLimiter(2).WithTimeout(300)

	var wg sync.WaitGroup
	wg.Add(5)

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			l.Wait()
		}()
	}
	time.Sleep(500 * time.Millisecond)
	wg.Wait()
	l.Finish()
	l.Finish()
	assert.Equal(t, 3, l.count)
	assert.Equal(t, 0, l.waitList.Len())
}
