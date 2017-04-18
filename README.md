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

## Fake subscription

This version aims to fix the issues presented before. It's called fake because it is still not
fetching real rss feeds, but it works properly.

To fix bug 1 we need to change the `closed bool` and `err error` values, shared by client code
and internal loop, to communication channels. In fact `closing chan chan error`, a channel
that expects a channel back, will work like a simple request-response pattern. The internal
loop, like a server, provides a channel where it will be listening for requests to close.
The client will send a closing request in the shape of an `chan error` which should be used
by the server to send an acknowledgement back to the client, in addition to a possible error
value.

```
type sub struct {
	fetcher Fetcher
	updates chan Item
	closing chan chan error
}
```

When `Close()` from a subscription is called, an error channel will be created and sent to the
`s.closing` channel. The return will block until it gets a response back from the error channel
just created.

```
func (s *sub) Close() error {
	errc := make(chan error)
	s.closing <- errc
	return <-errc             // blocks here
}
```

And from inside the loop:
```
func (s *sub) loop() {

    ...

    select {
    case errc := <-s.closing: // In case we receive a channel from s.closing
        errc <- err           // we send the current error value through,
        close(s.updates)      // close the subscription
        return                // and return

    ...
}
```

For bug 2, instead of using a explicit sleep call between fetches, the time for the next
fetch will be calculated with `time.After(fetchDelay)`, which returns a channel. 
```
func (s *sub) loop() {
    ...
	var next time.Time
    ...
    for {
        var fetchDelay time.Duration
        if now := time.Now(); next.After(now) {
            fetchDelay = next.Sub(now)
        }
        startFetch := time.After(fetchDelay)        // "timer" channel
```

Then, a value will be send to this channel when the timer set expires.

```
func (s *sub) loop() {

    ...

    select {

    ...

    case <-startFetch:                              // timer expires: receives value
        var fetched []Item
        fetched, next, err = s.fetcher.Fetch()      // fetch items
        if err != nil {
            next = time.Now().Add(10 * time.Second) // set next delay in case of an error
            break
        }
        pending = append(pending, fetched...)       // append results

    ...

    }
}
```

Finally, for bug 3 we want to deliver the items to the client channel. A first simple
approach where the first item from the `pending` slice is sent to `s.updates` is also
buggy. If `pending` has no items, `pending[0]` would panic with index out of range.

The improved version uses the nil channel pattern. An attempt to receive an item from
a `nil` channel doesn't panic, it blocks the same way a defined channel would when no
value is available - *it's a feature, not a bug*. Inside of a select-case, the `nil`
channel will just be ignored.

```
func (s *sub) loop() {

        ...

		var updates chan Item      // nil: declared but not set
		if len(pending) > 0 {
			first = pending[0]
			updates = s.updates    // define updates
		}

        ...

        select {
		case updates <- first:     // fired only if updates is defined
			pending = pending[1:]
		}

        ...
}
```

So the trick here is to keep the channel enabled (defined, not `nil`) only if there is
a `first` item to be sent. If more than one item is fetched, the for loop will likely
select the same case again and again to send all items to `s.updates` until have them
all delivered, then blocking while waiting for the next fetch delay expire, or for a
close request from client.

### Merge subscriptions

Running `go run -race *.go` still reports data races. `naiveMerge` still makes it
possible to access its `updates` channel directly from different routines: in `Close`
and inside `NaiveMerge` loop. The solution is on the same line of the ones applied
to subscription, unsynchonized access to values should be converted to communication
by channels.

```
func Merge(subs ...Subscription) Subscription {

    ...

	for _, sub := range subs {            // for each subscription
		go func(s Subscription) {         // create a goroutine
			for {                         // in a loop:
                var it Item
                select {                  // blocks waiting for
                case it = <-s.Updates():  // an update from subscription
                case <-m.quit:            // or a quit request
                    m.errs <- s.Close()
                    return
                }

                select {                  // if got an Item
                case m.updates <- it:     // send to merged sub's updates
                case <-m.quit:            // or receives a quit, whatever comes first
                    m.errs <- s.Close()
                    return
                }

      ...
}
```

The `Merge` function returns `merge` struct, which is a `Subscription`. So needs to
to implement `Close()` and `Updates()`. Now `Close()` will not attempt to close the
subs' channels directly, it will close its own `quit` channel instead.

```
func (m *merge) Close() (err error) {
	close(m.quit)
	for _ = range m.subs {
		if e := <-m.errs; e != nil {
			err = e
		}
	}
	close(m.updates)
	return
}
```

When a channel is closed, the receivers, after reading all remaining values, won't
be able to block waiting for more values. The lines `case <-m.quit:` from `Merge`'s
loop will receive an empty value, calling `Close()` for each subscription and
sending the errors back to the `m.errs`. Finally, `m.updates` can be safely closed.
