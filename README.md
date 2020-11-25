# concurrency-limiter

[![GoDoc reference example](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/vivek-ng/concurrency-limiter)
[![GoReportCard example](https://goreportcard.com/badge/github.com/nanomsg/mangos)](https://goreportcard.com/report/github.com/vivek-ng/concurrency-limiter)
[![codecov](https://codecov.io/gh/vivek-ng/concurrency-limiter/branch/main/graph/badge.svg?token=UFN7OqUNDH)](https://codecov.io/gh/vivek-ng/concurrency-limiter)

## About

concurrency-limiter allows you to limit the number of goroutines accessing a resource with support for
timeouts , dynamic priority of goroutines and context cancellation of goroutines.

### Installation

To install concurrency-limiter:

```
go get github.com/vivek-ng/concurrency-limiter
```

Then import concurrency-limiter to use it

```go
    import(
        github.com/vivek-ng/concurrency-limiter/limiter
    )

    nl := limiter.New(3)
    ctx := context.Background()
    nl.Wait(ctx)
    Execute......
    nl.Finish()

```

## Examples

### Limiter

```go
    nl := limiter.New(3)
    ctx := context.Background()
    nl.Wait(ctx)
    Execute......
    nl.Finish()
```
In the above example , there can be a maximum of 3 goroutines accessing a resource concurrently. The other goroutines are added to the waiting list and are removed and given a 
chance to access the resource in the FIFO order. If the context is cancelled , the goroutine is removed from the waitlist.

### Limiter with Timeout

```go
    nl := limiter.New(3,
    WithTimeout(10),
    )
    ctx := context.Background()
    nl.Wait(ctx)
    Execute......
    nl.Finish()
```
In the above example , the goroutines will wait for a maximum of 10 milliseconds. Goroutines will be removed from the waitlist after 10 ms even if the 
number of concurrent goroutines is greater than the limit specified.

### Priority Limiter

```go
    nl := priority.NewLimiter(3)
    ctx := context.Background()
    nl.Wait(ctx , priority.High)
    Execute......
    nl.Finish()
```

In Priority Limiter , goroutines with higher priority will be given preference to be removed from the waitlist. For instance in the above example , the goroutine will be
given the maximum preference because it is of high priority. In the case of tie between the priorities , the goroutines will be removed from the waitlist in the FIFO order.

### Priority Limiter with Dynamic priority

```go
    nl := priority.NewLimiter(3,
    WithDynamicPriority(5),
    )
    ctx := context.Background()
    nl.Wait(ctx , priority.Low)
    Execute......
    nl.Finish()
```
In Dynamic Priority Limiter , the goroutines with lower priority will get their priority increased periodically by the time period specified. For instance in the above example , the goroutine will get it's priority increased every 5 ms. This will ensure that goroutines with lower priority do not suffer from starvation. It's highly recommended to use Dynamic Priority Limiter to avoid starving low priority goroutines.

### Priority Limiter with Timeout

```go
    nl := priority.NewLimiter(3,
    WithTimeout(30),
    WithDynamicPriority(5),
    )
    ctx := context.Background()
    nl.Wait(ctx , priority.Low)
    Execute......
    nl.Finish()
```
This is similar to the timeouts in the normal limiter. In the above example , goroutines will wait a maximum of 30 milliseconds. The low priority goroutines will get their
priority increased every 5 ms.

### Executable Function

```go
    nl := priority.NewLimiter(3)
    ctx := context.Background()
    nl.Execute(ctx , priority.Low , func()error {
        return sendMetrics()
    })
```

Executable function will allow you to wrap your function and execute them with concurrency limit. This function is a wrapper on top of the Wait() and Finish() functions.

### Contribution

Please feel free to open up issues , create PRs for bugs/features. All contributions are welcome :)

