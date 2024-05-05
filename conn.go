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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// channel name should NOT match the negation of valid characters
var channelRegexp = regexp.MustCompile("[^a-zA-Z0-9 ]+")

type Conn struct {
	sync.Mutex
	peer        *WebRTCPeer
	wsConn      *websocket.Conn
	channelName string
	infoChan    chan string
	quitchan    chan struct{}
	logger      *slog.Logger
	hasClosed   bool

	clientID    string
	isPublisher bool
}

func NewConn(ws *websocket.Conn) *Conn {
	c := &Conn{}
	c.infoChan = make(chan string)
	c.quitchan = make(chan struct{})
	c.logger = slog.With("remote_addr", ws.RemoteAddr())
	c.wsConn = ws

	return c
}

func (c *Conn) setupSessionPublisher(offer webrtc.SessionDescription) error {

	answer, err := c.peer.SetupPublisher(offer, c.rtcStateChangeHandler, c.rtcTrackHandlerPublisher, c.onIceCandidate)
	if err != nil {
		return err
	}

	j, err := json.Marshal(answer.SDP)
	if err != nil {
		return err
	}
	err = c.writeMsg(wsMsg{Key: "sd_answer", Value: j})
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) setupSessionSubscriber() error {

	channel := reg.GetChannel(c.channelName)
	if channel == nil {
		return fmt.Errorf("channel %q not found", c.channelName)
	}

	answer, err := c.peer.SetupSubscriber(channel, c.rtcStateChangeHandler, c.onIceCandidate)
	if err != nil {
		return err
	}

	j, err := json.Marshal(answer.SDP)
	if err != nil {
		return err
	}
	err = c.writeMsg(wsMsg{Key: "sd_answer", Value: j})
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) connectPublisher(cmd CmdConnect) error {

	if c.peer.pc == nil {
		return fmt.Errorf("webrtc session not established")
	}

	if cmd.Channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}

	if channelRegexp.MatchString(cmd.Channel) {
		return fmt.Errorf("channel name must contain only alphanumeric characters")
	}

	if publisherPassword != "" && cmd.Password != publisherPassword {
		return fmt.Errorf("incorrect password")
	}

	c.channelName = cmd.Channel
	c.logger.Info("setting up publisher for channel", "channel", c.channelName)

	localTrack := <-c.peer.localTrackChan
	c.logger.Info("publisher has localTrack")

	if err := reg.AddPublisher(c.channelName, localTrack); err != nil {
		return err
	}

	return nil
}

func (c *Conn) Close() {
	c.logger.Debug("close called")
	c.Lock()
	defer c.Unlock()
	if c.hasClosed {
		return
	}
	if c.isPublisher {
		reg.RemovePublisher(c.channelName)
	} else {
		reg.RemoveSubscriber(c.channelName, c.clientID)
	}
	if c.peer.pc != nil {
		c.peer.pc.Close()
	}
	if c.wsConn != nil {
		c.wsConn.Close()
	}
	c.hasClosed = true
}

func (c *Conn) writeMsg(val interface{}) error {
	c.Lock()
	defer c.Unlock()
	j, err := json.Marshal(val)
	if err != nil {
		return err
	}
	c.logger.Debug("write message", "msg", string(j))
	if err = c.wsConn.WriteMessage(websocket.TextMessage, j); err != nil {
		return err
	}

	return nil
}

// WebRTC callback function
func (c *Conn) rtcTrackHandlerPublisher(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

	// Create a local track, all our SFU clients will be fed via this track
	localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "audio", "babelcast")
	if newTrackErr != nil {
		panic(newTrackErr)
	}

	c.logger.Debug("trackhandler sending localtrack")
	c.peer.localTrackChan <- localTrack
	c.logger.Debug("trackhandler sent localtrack")

	rtpBuf := make([]byte, 1400)
	for {
		i, _, readErr := remoteTrack.Read(rtpBuf)
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				c.logger.Error("remoteTrack.Read error", "err", readErr)
			}
			return
		}

		// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
		_, err := localTrack.Write(rtpBuf[:i])
		if err != nil {
			c.logger.Error("localTrack.write error", "err", err)
			if !errors.Is(err, io.ErrClosedPipe) {
				return
			}
		}
	}
}

// WebRTC callback function
func (c *Conn) rtcStateChangeHandler(connectionState webrtc.ICEConnectionState) {
	switch connectionState {
	case webrtc.ICEConnectionStateConnected:
		c.logger.Info("ice connected")
		c.logger.Debug("remote SDP", "sdp", c.peer.pc.RemoteDescription().SDP)
		c.logger.Debug("local SDP", "sdp", c.peer.pc.LocalDescription().SDP)
		c.infoChan <- "ice connected"

	case webrtc.ICEConnectionStateDisconnected:
		c.logger.Info("ice disconnected")
		c.infoChan <- "ice disconnected"
	}
}

// WebRTC callback function
func (c *Conn) onIceCandidate(candidate *webrtc.ICECandidate) {
	if candidate == nil {
		return
	}

	j, err := json.Marshal(candidate.ToJSON())
	if err != nil {
		c.logger.Error("marshal error", "err", err.Error())
		return
	}

	c.logger.Debug("ICE candidate", "candidate", j)

	m := wsMsg{Key: "ice_candidate", Value: j}
	err = c.writeMsg(m)
	if err != nil {
		c.logger.Error("writemsg error", "err", err.Error())
		return
	}
}
