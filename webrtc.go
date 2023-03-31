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
	"fmt"

	"github.com/pion/webrtc/v3"
)

func NewPCPublisher(offerSd string, onStateChange func(connectionState webrtc.ICEConnectionState), onTrack func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)) (*webrtc.PeerConnection, error) {

	var err error
	var pc *webrtc.PeerConnection

	// Create a new RTCPeerConnection
	pc, err = webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Allow us to receive 1 audio track
	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}

	pc.OnICEConnectionStateChange(onStateChange)

	pc.OnTrack(onTrack)

	// Set the remote SessionDescription
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSd,
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		return nil, err
	}

	return pc, nil
}

func NewPCSubscriber(offerSd string, channel *Channel, onStateChange func(connectionState webrtc.ICEConnectionState)) (*webrtc.PeerConnection, error) {

	var err error
	var pc *webrtc.PeerConnection

	// Create a new RTCPeerConnection
	pc, err = webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	rtpSender, err := pc.AddTrack(channel.LocalTrack)
	if err != nil {
		return nil, fmt.Errorf("rtcPeer.AddTrack err '%w'", err)
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			_, _, rtcpErr := rtpSender.Read(rtcpBuf)
			if rtcpErr != nil {
				fmt.Printf("rtpSender.Read err '%s'\n", rtcpErr)
				return
			}
		}
	}()

	pc.OnICEConnectionStateChange(onStateChange)

	// Set the remote SessionDescription
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSd,
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		return nil, err
	}

	return pc, nil
}
