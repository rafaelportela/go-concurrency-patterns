package main

import (
	"fmt"
	"math/rand"
	"time"
)

type Item struct {
	Title, Channel, GUID string // subset of RSS fields
}

type Fetcher interface {
	// Fetches items for a given uri and returns the time when the next
	// fetch should be attempted.
	Fetch() (items []Item, next time.Time, err error)
}

// Subscription delivers Items over a channel.
// Close cancels the subscription, closes the Updates channel and
// returns the last fetch error, if any.
type Subscription interface {
	Updates() <-chan Item // stream of Items
	Close() error         // close the stream
}

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

func (s *sub) loopCloseOnly() {
	var err error // set when fetch fails
	for {
		select {
		case errc := <-s.closing:
			errc <- err
			close(s.updates)
			return
		}
	}
}

func (s *sub) loop() {
	s.loopCloseOnly()
}

// loop fetches Items using s.fetcher and send them on s.updates
// Exits when s.Close is called
//func (s *sub) loop() {
//	for {
//		if s.closed {
//			close(s.updates)
//			return
//		}
//		items, next, err := s.fetcher.Fetch()
//		if err != nil {
//			s.err = err
//			time.Sleep(10 * time.Second)
//			continue
//		}
//		for _, item := range items {
//			s.updates <- item
//		}
//		if now := time.Now(); next.After(now) {
//			time.Sleep(next.Sub(now))
//		}
//}

// goroutines may block forever on m.updates if the receiver
// stops receiving.
type naiveMerge struct {
	subs    []Subscription
	updates chan Item
}

func NaiveMerge(subs ...Subscription) Subscription {
	m := &naiveMerge{
		subs:    subs,
		updates: make(chan Item),
	}
	for _, sub := range subs {
		go func(s Subscription) {
			for it := range s.Updates() {
				m.updates <- it
			}
		}(sub)
	}
	return m
}

func (m *naiveMerge) Close() (err error) {
	for _, sub := range m.subs {
		if e := sub.Close(); err == nil && e != nil {
			err = e
		}
	}
	close(m.updates)
	return
}

func (m *naiveMerge) Updates() <-chan Item {
	return m.updates
}

func Fetch(domain string) Fetcher {
	return fakeFetch(domain)
}

func fakeFetch(domain string) Fetcher {
	return &fakeFetcher{channel: domain}
}

type fakeFetcher struct {
	channel string
	items   []Item
}

var FakeDuplicates bool

func (f *fakeFetcher) Fetch() (items []Item, next time.Time, err error) {
	now := time.Now()
	next = now.Add(time.Duration(rand.Intn(5)) * 500 * time.Millisecond)
	item := Item{
		Channel: f.channel,
		Title:   fmt.Sprintf("Item %d", len(f.items)),
	}
	item.GUID = item.Channel + "/" + item.Title
	f.items = append(f.items, item)
	if FakeDuplicates {
		items = f.items
	} else {
		items = []Item{item}
	}
	return
}

func main() {
	// Subscribe to some feeds and create a merged update stream
	merged := NaiveMerge(
		Subscribe(Fetch("blog.goland.org")),
		Subscribe(Fetch("googleblog.blogspot.com")),
		Subscribe(Fetch("googledevelopers.blogspot.com")))

	// Close the subscription after some time
	time.AfterFunc(3*time.Second, func() {
		fmt.Println("closed:", merged.Close())
	})

	// Print the stream
	for it := range merged.Updates() {
		fmt.Println(it.Channel, it.Title)
	}

	panic("Show me the stacks")
}
