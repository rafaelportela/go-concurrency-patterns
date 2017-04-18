package main

type merge struct {
	subs    []Subscription
	updates chan Item
	quit    chan struct{}
	errs    chan error
}

func Merge(subs ...Subscription) Subscription {
	m := &merge{
		subs:    subs,
		updates: make(chan Item),
		quit:    make(chan struct{}),
		errs:    make(chan error),
	}

	for _, sub := range subs {
		go func(s Subscription) {
			for {
				var it Item
				select {
				case it = <-s.Updates():
				case <-m.quit:
					m.errs <- s.Close()
					return
				}

				select {
				case m.updates <- it:
				case <-m.quit:
					m.errs <- s.Close()
					return
				}
			}
		}(sub)
	}

	return m
}

func (m *merge) Updates() <-chan Item {
	return m.updates
}

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
