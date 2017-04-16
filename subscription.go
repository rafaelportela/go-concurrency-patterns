package main

import (
	"time"
)

// returns a new Subscription using Fetcher to fetch Items.
func Subscribe(fetcher Fetcher) Subscription {
	s := &sub{
		fetcher: fetcher,
		updates: make(chan Item),
		closing: make(chan chan error),
	}
	go s.loop()
	return s
}

// sub implements the subscription interface
type sub struct {
	fetcher Fetcher         // fetches Items
	updates chan Item       // delivers Items to the user
	closing chan chan error // for Close
}

func (s *sub) Updates() <-chan Item {
	return s.updates
}

func (s *sub) Close() error {
	errc := make(chan error)
	s.closing <- errc
	return <-errc
}

// mergedLoop: it combines loopFetchOnly, loopSendOnly
// and loopCloseOnly
func (s *sub) loop() {
	var pending []Item
	var next time.Time
	var err error

	for {
		var fetchDelay time.Duration
		if now := time.Now(); next.After(now) {
			fetchDelay = next.Sub(now)
		}

		// timer setup: returns chan that will receive current time after delay
		startFetch := time.After(fetchDelay)

		var first Item
		var updates chan Item
		if len(pending) > 0 {
			first = pending[0]
			updates = s.updates
		}

		select {
		case errc := <-s.closing:
			errc <- err
			close(s.updates)
			return
		case <-startFetch:
			var fetched []Item
			fetched, next, err = s.fetcher.Fetch()
			if err != nil {
				next = time.Now().Add(10 * time.Second)
				break
			}
			pending = append(pending, fetched...)
		case updates <- first:
			pending = pending[1:]
		}
	}
}
