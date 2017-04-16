package main

import (
	"fmt"
	"time"
)

func main() {

	fmt.Println("Start of main")
	// Subscribe to some feeds and create a merged update stream
	merged := NaiveMerge(
		Subscribe(Fetch("blog.goland.org")),
		Subscribe(Fetch("googleblog.blogspot.com")),
		Subscribe(Fetch("googledevelopers.blogspot.com")))

	fmt.Println("Scheduling to close all subscriptions in 3 secs")
	// Close the subscription after some time
	time.AfterFunc(3*time.Second, func() {
		fmt.Println("Merged subsription closed. Errors: ", merged.Close())
	})

	fmt.Println("Blocking to wait for items from merged subscriptions")
	// Print the stream.
	// When updates channel is closed, range understands that and for loop ends.
	for it := range merged.Updates() {
		fmt.Println(it.Channel, it.Title)
	}

	fmt.Println("End of main")
	panic("Show me the stacks")
}
