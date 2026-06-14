package proxy

import "sync/atomic"

// RoundRobin selects upstream URLs in rotation.
type RoundRobin struct {
	urls []string
	next uint64
}

func NewRoundRobin(urls []string) *RoundRobin {
	if len(urls) == 0 {
		return &RoundRobin{urls: []string{""}}
	}
	return &RoundRobin{urls: urls}
}

func (rr *RoundRobin) Next() string {
	if len(rr.urls) == 1 {
		return rr.urls[0]
	}
	i := atomic.AddUint64(&rr.next, 1)
	return rr.urls[i%uint64(len(rr.urls))]
}
