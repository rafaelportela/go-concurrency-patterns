package main

import (
	"fmt"
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

func main() {

	// Subscribe to some feeds and create a merged update stream
	merged := NaiveMerge(
		NaiveSubscribe(Fetch("blog.goland.org")),
		NaiveSubscribe(Fetch("googleblog.blogspot.com")),
		NaiveSubscribe(Fetch("googledevelopers.blogspot.com")))

	// Close the subscription after some time
	time.AfterFunc(3*time.Second, func() {
		fmt.Println("Merged subsription closed. Errors: ", merged.Close())
	})

	// Print the stream.
	// When updates channel is closed, range understands that and for loop ends.
	for it := range merged.Updates() {
		fmt.Println(it.Channel, it.Title)
	}

	time.Sleep(1 * time.Second)
	fmt.Println("End of main")

	panic("Show me the stacks")
}
