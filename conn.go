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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

const (
	rtcpPLIInterval = time.Second * 3
)

// channel name should NOT match the negation of valid characters
var channelRegexp = regexp.MustCompile("[^a-zA-Z0-9 ]+")

type Conn struct {
	sync.Mutex

	peer *WebRTCPeer

	wsConn *websocket.Conn

	channelName string

	errChan  chan error
	infoChan chan string

	logger *slog.Logger

	hasClosed bool
}

func NewConn(ws *websocket.Conn) *Conn {
	c := &Conn{}
	c.errChan = make(chan error)
	c.infoChan = make(chan string)
	c.logger = slog.With("remote_addr", ws.RemoteAddr())
	// wrap Gorilla conn with our conn so we can extend functionality
	c.wsConn = ws

	return c
}

func (c *Conn) setupSessionPublisher(offer webrtc.SessionDescription) error {

	c.logger = c.logger.With("client_type", "publisher")

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

	c.logger = c.logger.With("client_type", "subscriber")

	channel := reg.GetChannel(c.channelName)
	if channel == nil {
		return fmt.Errorf("channel '%s' not found", c.channelName)
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

	c.Lock()
	c.channelName = cmd.Channel
	c.Unlock()
	c.logger.Info("setting up publisher for channel", "channel", c.channelName)

	localTrack := <-c.peer.localTrackChan
	c.logger.Info("publisher has localTrack")

	if err := reg.AddPublisher(c.channelName, localTrack); err != nil {
		return err
	}

	return nil
}

func (c *Conn) Close() {
	c.Lock()
	defer c.Unlock()
	if c.hasClosed {
		return
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
	j, err := json.Marshal(val)
	if err != nil {
		return err
	}
	c.logger.Debug("write message", "msg", string(j))
	c.Lock()
	defer c.Unlock()
	if err = c.wsConn.WriteMessage(websocket.TextMessage, j); err != nil {
		return err
	}

	return nil
}

// WebRTC callback function
func (c *Conn) rtcTrackHandlerPublisher(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

	// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
	// This can be less wasteful by processing incoming RTCP events, then we would emit a NACK/PLI when a viewer requests it
	go func() {
		ticker := time.NewTicker(rtcpPLIInterval)
		for range ticker.C {
			err := c.peer.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(remoteTrack.SSRC())}})
			if err != nil {
				c.logger.Error("WriteRTCP error", "err", err)
				return
			}
		}
	}()

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

	//var err error

	switch connectionState {
	case webrtc.ICEConnectionStateConnected:
		c.logger.Info("ice connected")
		c.logger.Debug("remote SDP", "sdp", c.peer.pc.RemoteDescription().SDP)
		c.logger.Debug("local SDP", "sdp", c.peer.pc.LocalDescription().SDP)
		c.infoChan <- "ice connected"

	case webrtc.ICEConnectionStateDisconnected:
		c.logger.Info("ice disconnected")
		c.Close()

		// non blocking channel write, as receiving goroutine may already have quit
		select {
		case c.infoChan <- "ice disconnected":
		default:
		}
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

func (c *Conn) LogHandler(ctx context.Context) {
	defer c.logger.Info("log goroutine quit")
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-c.errChan:
			j, err := json.Marshal(err.Error())
			if err != nil {
				c.logger.Error("marshal error", "err", err.Error())
			}
			m := wsMsg{Key: "error", Value: j}
			err = c.writeMsg(m)
			if err != nil {
				c.logger.Error("writemsg error", "err", err.Error())
			}
			// end the WS session on error
			c.Close()
		case info := <-c.infoChan:
			j, err := json.Marshal(info)
			if err != nil {
				c.logger.Error("marshal error", "err", err.Error())
			}
			m := wsMsg{Key: "info", Value: j}
			err = c.writeMsg(m)
			if err != nil {
				c.logger.Error("writemsg error", "err", err.Error())
			}
		}
	}
}

func (c *Conn) PingHandler(ctx context.Context) {
	defer c.logger.Info("ws ping goroutine quit")
	pingCh := time.Tick(PingInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-pingCh:
			c.Lock()
			// WriteControl can be called concurrently
			err := c.wsConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(WriteWait))
			if err != nil {
				c.Unlock()
				c.logger.Error("ping client error", "err", err.Error())
				return
			}
			c.Unlock()
		}
	}
}

/*
func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
*/
