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
	Active          bool
}

/*
type Client struct {
	Username string
}
*/

func NewRegistry() *Registry {
	r := &Registry{}
	r.Channels = make(map[string]*Channel)
	return r
}

func (r *Registry) AddPublisher(channelName string) error {
	var channel *Channel
	var ok bool
	r.Lock()
	if channel, ok = r.Channels[channelName]; ok {
		if channel.PublisherCount != 0 {
			return fmt.Errorf("channel '%s' is already in use", channelName)
		}
		channel.Active = true
	} else {
		r.Channels[channelName] = &Channel{PublisherCount: 1, Active: true}
	}
	r.Unlock()
	return nil
}

func (r *Registry) AddSubscriber(channelName string) error {
	var channel *Channel
	var ok bool

	r.Lock()
	if channel, ok = r.Channels[channelName]; ok && channel.Active {
		channel.SubscriberCount++
	} else {
		return fmt.Errorf("channel '%s' not ready", channelName)
	}
	r.Unlock()
	return nil
}

func (r *Registry) RemovePublisher(channelName string) {
	r.Lock()
	if channel, ok := r.Channels[channelName]; ok {
		channel.PublisherCount--
		if channel.PublisherCount == 0 {
			channel.Active = false
		}
	}
	r.Unlock()
}

func (r *Registry) RemoveSubscriber(channelName string) {
	r.Lock()
	if channel, ok := r.Channels[channelName]; ok {
		channel.SubscriberCount--
	}
	r.Unlock()
}

func (r *Registry) GetChannels() []string {
	r.Lock()
	defer r.Unlock()
	channels := make([]string, 0)
	for name, c := range r.Channels {
		if c.Active {
			channels = append(channels, name)
		}
	}
	return channels
}
