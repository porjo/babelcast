package main

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/pion/webrtc/v3"
)

// keep track of which channels are being used
// only permit one publisher per channel
type Registry struct {
	sync.Mutex
	channels map[string]*Channel
}

type Channel struct {
	PublisherCount  int
	SubscriberCount int
	Active          bool
	LocalTrack      *webrtc.TrackLocalStaticRTP
}

func NewRegistry() *Registry {
	r := &Registry{}
	r.channels = make(map[string]*Channel)
	return r
}

func (r *Registry) AddPublisher(channelName string, localTrack *webrtc.TrackLocalStaticRTP) error {
	var channel *Channel
	var ok bool
	r.Lock()
	defer r.Unlock()
	if channel, ok = r.channels[channelName]; ok {
		if channel.PublisherCount > 0 {
			return fmt.Errorf("channel '%s' is already in use", channelName)
		}
		channel.PublisherCount++
		channel.LocalTrack = localTrack
		channel.Active = true
	} else {
		channel = &Channel{PublisherCount: 1, Active: true, LocalTrack: localTrack}
		r.channels[channelName] = channel
	}
	slog.Info("publisher added", "channel", channelName, "count", channel.PublisherCount)
	return nil
}

func (r *Registry) AddSubscriber(channelName string) error {
	var channel *Channel
	var ok bool

	r.Lock()
	defer r.Unlock()
	if channel, ok = r.channels[channelName]; ok && channel.Active {
		channel.SubscriberCount++
		slog.Info("subscriber added", "channel", channelName, "count", channel.SubscriberCount)
	} else {
		return fmt.Errorf("channel '%s' not ready", channelName)
	}
	return nil
}

func (r *Registry) RemovePublisher(channelName string) {
	r.Lock()
	defer r.Unlock()
	if channel, ok := r.channels[channelName]; ok {
		channel.PublisherCount--
		if channel.PublisherCount == 0 {
			channel.Active = false
			slog.Info("publisher removed", "channel", channelName, "count", channel.PublisherCount)
		}
	}
}

func (r *Registry) RemoveSubscriber(channelName string) {
	r.Lock()
	defer r.Unlock()
	if channel, ok := r.channels[channelName]; ok {
		channel.SubscriberCount--
		slog.Info("subscriber removed", "channel", channelName, "count", channel.SubscriberCount)
	}
}

func (r *Registry) GetChannels() []string {
	r.Lock()
	defer r.Unlock()
	channels := make([]string, 0)
	for name, c := range r.channels {
		if c.Active {
			channels = append(channels, name)
		}
	}
	return channels
}

func (r *Registry) GetChannel(channelName string) *Channel {
	r.Lock()
	defer r.Unlock()
	for name, c := range r.channels {
		if c.Active && name == channelName {
			return c
		}
	}
	return nil
}
