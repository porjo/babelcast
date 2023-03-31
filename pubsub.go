package main

// credit to Eli Bendersky: https://eli.thegreenplace.net/2020/pubsub-using-channels-in-go/

import (
	"sync"
)

type Pubsub struct {
	mu     sync.RWMutex
	subs   map[string][]BytesChan
	closed bool
}

type BytesChan chan []byte

func NewPubsub() *Pubsub {
	ps := &Pubsub{}
	ps.subs = make(map[string][]BytesChan)
	return ps
}

func (ps *Pubsub) Publish(topic string, msg []byte) {
	//	fmt.Printf("publish enter topic '%s'\n", topic)
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return
	}

	if topic == "" || msg == nil || len(msg) == 0 {
		return
	}

	for _, ch := range ps.subs[topic] {
		ch <- msg
	}
}

func (ps *Pubsub) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.closed {
		ps.closed = true
		for _, topics := range ps.subs {
			for _, ch := range topics {
				close(ch)
			}
		}
	}
}

func (ps *Pubsub) Subscribe(topic string) BytesChan {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(BytesChan, 1)
	ps.subs[topic] = append(ps.subs[topic], ch)
	return ch
}
