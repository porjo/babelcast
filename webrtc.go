/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"math/rand"

	//"github.com/pion/ice"
	"github.com/pion/webrtc/v2"
)

type WebRTCPeer struct {
	pc    *webrtc.PeerConnection
	track *webrtc.Track
}

func (w *WebRTCPeer) Close() error {
	return w.pc.Close()
}

func NewPC(offerSd string, onStateChange func(connectionState webrtc.ICEConnectionState), onTrack func(track *webrtc.Track, receiver *webrtc.RTPReceiver)) (*WebRTCPeer, error) {

	var err error
	var pc *webrtc.PeerConnection
	var opusTrack *webrtc.Track
	var peer *WebRTCPeer

	// Register only audio codec (Opus)
	m := webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// Create a new RTCPeerConnection
	pc, err = api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	pc.OnICEConnectionStateChange(onStateChange)

	pc.OnTrack(onTrack)
	// Create a audio track
	opusTrack, err = pc.NewTrack(webrtc.DefaultPayloadTypeOpus, rand.Uint32(), "audio", "babelcast")
	if err != nil {
		return nil, err
	}
	_, err = pc.AddTrack(opusTrack)
	if err != nil {
		return nil, err
	}

	// Set the remote SessionDescription
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSd,
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		return nil, err
	}

	peer = &WebRTCPeer{pc: pc, track: opusTrack}

	return peer, nil
}
