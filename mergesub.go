package main

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
