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

	const maxPending = 10
	type fetchResult struct {
		fetched []Item
		next    time.Time
		err     error
	}
	var fetchDone chan fetchResult

	var pending []Item
	var next time.Time
	var err error
	var seen = make(map[string]bool)

	for {
		var fetchDelay time.Duration
		if now := time.Now(); next.After(now) {
			fetchDelay = next.Sub(now)
		}

		var startFetch <-chan time.Time
		if fetchDone == nil && len(pending) < maxPending {
			startFetch = time.After(fetchDelay)
		}

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
			fetchDone = make(chan fetchResult, 1)
			go func() {
				fetched, next, err := s.fetcher.Fetch()
				fetchDone <- fetchResult{fetched, next, err}
			}()
		case result := <-fetchDone:
			fetchDone = nil
			fetched := result.fetched
			next, err = result.next, result.err
			if err != nil {
				next = time.Now().Add(10 * time.Second)
				break
			}
			for _, item := range fetched {
				if !seen[item.GUID] {
					pending = append(pending, item)
					seen[item.GUID] = true
				}
			}
		case updates <- first:
			pending = pending[1:]
		}
	}
}
