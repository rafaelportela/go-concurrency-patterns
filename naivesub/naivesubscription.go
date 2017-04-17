package main

import (
	"time"
)

func NaiveSubscribe(fetcher Fetcher) Subscription {
	s := &naiveSub{
		fetcher: fetcher,
		updates: make(chan Item),
	}
	go s.loop()
	return s
}

// sub implements the subscription interface
type naiveSub struct {
	fetcher Fetcher
	updates chan Item
	closed  bool
	err     error
}

func (s *naiveSub) Updates() <-chan Item {
	return s.updates
}

func (s *naiveSub) Close() error {
	s.closed = true
	return s.err
}

func (s *naiveSub) loop() {
	for {
		if s.closed {
			close(s.updates)
			return
		}
		items, next, err := s.fetcher.Fetch()
		if err != nil {
			s.err = err
			time.Sleep(10 * time.Second)
			continue
		}
		for _, item := range items {
			s.updates <- item
		}
		if now := time.Now(); next.After(now) {
			time.Sleep(next.Sub(now))
		}
	}
}
