package main

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
)

// keep track of which channels are being used
// only permit one publisher per channel
type Registry struct {
	sync.Mutex
	channels map[string]*Channel
}

type Channel struct {
	LocalTrack *webrtc.TrackLocalStaticRTP

	Publisher   *Publisher
	Subscribers map[string]*Subscriber
}

type Publisher struct {
	ID string
}
type Subscriber struct {
	ID          string
	closeChanFn func()
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
	p := Publisher{}
	p.ID = uuid.NewString()
	if channel, ok = r.channels[channelName]; ok {
		if channel.Publisher != nil {
			return fmt.Errorf("channel %q is already in use", channelName)
		}
		channel.LocalTrack = localTrack
		channel.Publisher = &p
	} else {
		channel = &Channel{
			LocalTrack:  localTrack,
			Publisher:   &p,
			Subscribers: make(map[string]*Subscriber),
		}
		r.channels[channelName] = channel
	}
	slog.Info("publisher added", "channel", channelName)
	return nil
}

func (r *Registry) NewSubscriber() *Subscriber {
	s := &Subscriber{}
	s.ID = uuid.NewString()
	return s
}

func (r *Registry) AddSubscriber(channelName string, s *Subscriber) error {
	var channel *Channel
	var ok bool

	r.Lock()
	defer r.Unlock()
	if channel, ok = r.channels[channelName]; ok && channel.Publisher != nil {
		channel.Subscribers[s.ID] = s
		slog.Info("subscriber added", "channel", channelName, "count", len(channel.Subscribers))
	} else {
		return fmt.Errorf("channel %q not ready", channelName)
	}
	return nil
}

func (r *Registry) RemovePublisher(channelName string) {
	r.Lock()
	defer r.Unlock()
	if channel, ok := r.channels[channelName]; ok {
		channel.Publisher = nil
		for _, s := range channel.Subscribers {
			if s.closeChanFn != nil {
				// this needs to be in its own goroutine, otherwise the current goroutine ends prematurely (why?)
				go s.closeChanFn()
			}
		}
		slog.Info("publisher removed", "channel", channelName)
	}
}

func (r *Registry) RemoveSubscriber(channelName string, id string) {
	r.Lock()
	defer r.Unlock()
	if channel, ok := r.channels[channelName]; ok {
		delete(channel.Subscribers, id)
		slog.Info("subscriber removed", "channel", channelName, "count", len(channel.Subscribers))
	}
}

func (r *Registry) GetChannels() []string {
	r.Lock()
	defer r.Unlock()
	channels := make([]string, 0)
	for name, c := range r.channels {
		if c.Publisher != nil {
			channels = append(channels, name)
		}
	}
	return channels
}

func (r *Registry) GetChannel(channelName string) *Channel {
	r.Lock()
	defer r.Unlock()
	for name, c := range r.channels {
		if name == channelName && c.Publisher != nil {
			return c
		}
	}
	return nil
}
