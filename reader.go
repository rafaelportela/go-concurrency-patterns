package main

import (
	"fmt"
	"time"
)

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
