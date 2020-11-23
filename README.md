# concurrency-limiter

[![GoDoc reference example](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/vivek-ng/concurrency-limiter)
[![GoReportCard example](https://goreportcard.com/badge/github.com/nanomsg/mangos)](https://goreportcard.com/report/github.com/vivek-ng/concurrency-limiter)

## About

concurrency-limiter is a minimalistic package that allows you to limit the number of concurrent goroutines accessing a resource with support for
timeouts and priority of goroutines.

## Examples

### Limiter

```go
    nl := limiter.New(3)
    nl.Wait()
    Execute......
    nl.Finish()
```
In the above example , there can be a maximum of 3 goroutines accessing a resource concurrently. The other goroutines are added to the waiting list and are removed and given a 
chance to access the resource in the FIFO order.

### Limiter with Timeout

```go
    nl := limiter.New(3).WithTimeout(10)
    nl.Wait()
    Execute......
    nl.Finish()
```
In the above example , the goroutines will wait for a maximum of 10 milliseconds. Goroutines will be removed from the waitlist after 10 ms even if the 
number of concurrent goroutines is greater than the limit specified.

### Priority Limiter

```go
    nl := priority.NewLimiter(3)
    nl.Wait(constants.High)
    Execute......
    nl.Finish()
```

In Priority Limiter , goroutines with higher priority will be given preference to be removed from the waitlist. For instance in the above example , the goroutine will be
given the maximum preference because it is of high priority. In the case of tie between the priorities , the goroutines will be removed from the waitlist in the FIFO order.

### Priority Limiter with Dynamic priority

```go
    nl := priority.NewLimiter(3).WithDynamicPriority(5)
    nl.Wait(constants.Low)
    Execute......
    nl.Finish()
```
In Dynamic Priority Limiter , the goroutines with lower priority will get their priority increased periodically by the time period specified. For instance in the above example , the goroutine will get it's priority increased every 5 ms. This will ensure that goroutines with lower priority do not suffer from starvation. It's highly recommended to use Dynamic Priority Limiter to avoid starving low priority goroutines.