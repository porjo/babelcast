package main

import (
	"fmt"
	"sync"
)

// keep track of which channels are being used
// only permit one publisher per channel
type Registry struct {
	sync.Mutex
	Channels map[string]*Channel
}

type Channel struct {
	PublisherCount  int
	SubscriberCount int
}

/*
type Client struct {
	Username string
}
*/

func (r *Registry) AddPublisher(channelName string) error {
	var channel *Channel
	var ok bool
	r.Lock()
	if channel, ok = r.Channels[channelName]; ok && channel.PublisherCount != 0 {
		return fmt.Errorf("channel '%s' is already in use", channelName)
	} else {
		r.Channels[channelName] = &Channel{PublisherCount: 1}
	}
	r.Unlock()

	return nil
}

func (r *Registry) AddSubscriber(channelName string) error {
	var channel *Channel
	var ok bool

	r.Lock()
	if channel, ok = r.Channels[channelName]; ok {
		channel.SubscriberCount++
	} else {
		r.Channels[channelName] = &Channel{SubscriberCount: 1}
	}
	r.Unlock()

	return nil
}
