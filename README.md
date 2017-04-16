# Advanced Go concurrency patterns

Follow up on the code presented in the talk [Go Concurrency Patterns](https://www.youtube.com/watch?v=QDDwwePbDtw&t=456s)
from [Sameer Ajmani](https://twitter.com/sajma).

## Naive subscription

Here `naiveSub` implements `Subscription`, it has `Updates()` and `Close()` functions.
When created, a loop will run forever fetching items and sending them to `updates` channel.

You can run with `go run -race *.go`.

```
func (s *naiveSub) loop() {
	for {
		if s.closed {                           // bug 1: data race
			close(s.updates)
			return
		}
		items, next, err := s.fetcher.Fetch()
		if err != nil {
			s.err = err                         // bug 1: data race
			time.Sleep(10 * time.Second)        // bug 2: wasteful blocking 
			continue
		}
		for _, item := range items {
			s.updates <- item                   // bug 3: may block forever
		}
		if now := time.Now(); next.After(now) {
			time.Sleep(next.Sub(now))           // bug 2: wasteful blocking
		}
	}
}
```

Bug 1: Unsynchronized access to s.close and s.err - Two goroutines can access the same data
with no synchronization. This would be a bigger problem if two or more routines try to change
the variable data concurrently, but still it's a big warning.

Bug 2: time.Sleep may keep loop running - May not be considered a bug but it will keep the loop
blocked and waiting no matter what. Sort of resource leak: if the routine is closed, it will
be hanging for a while instead of been cleaned up immediately.

Bug 3: loop may block forever on s.updates - Sending an item to a channel blocks until there is
a channel ready to receive the value. If the client code closes the subscription, this line
may be blocked, waiting forever.

Solution: Use the for-select loop pattern to catch if:
* `Close` was called
* it's time to call `Fetch`
* send one item on `s.updates`
