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
	"hash/fnv"
	"io"
	"log"
	"os"
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

	errChan       chan error
	infoChan      chan string
	trackQuitChan chan struct{}

	logger *log.Logger

	hasClosed bool
}

func NewConn(ws *websocket.Conn) *Conn {
	c := &Conn{}
	c.errChan = make(chan error)
	c.infoChan = make(chan string)
	c.trackQuitChan = make(chan struct{})
	c.logger = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	// wrap Gorilla conn with our conn so we can extend functionality
	c.wsConn = ws

	return c
}

func (c *Conn) Log(format string, v ...interface{}) {
	id := fmt.Sprintf("WS %x", c.wsConn.RemoteAddr())
	c.logger.Printf(id+": "+format, v...)
}

func (c *Conn) setupSessionPublisher(ctx context.Context, offer webrtc.SessionDescription) error {

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

func (c *Conn) setupSessionSubscriber(ctx context.Context) error {

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

func (c *Conn) connectPublisher(ctx context.Context, cmd CmdConnect) error {

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
	c.Log("setting up publisher for channel '%s'\n", c.channelName)

	localTrack := <-c.peer.localTrackChan
	c.Log("publisher has localTrack\n")

	if err := reg.AddPublisher(c.channelName, localTrack); err != nil {
		return err
	}

	return nil
}

func (c *Conn) connectSubscriber(ctx context.Context, cmd CmdConnect) error {

	if c.peer.pc == nil {
		return fmt.Errorf("webrtc session not established")
	}

	if cmd.Channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}
	if channelRegexp.MatchString(cmd.Channel) {
		return fmt.Errorf("channel name must contain only alphanumeric characters")
	}

	c.Log("setting up subscriber for channel '%s'\n", c.channelName)

	return nil
}

func (c *Conn) Close() {
	c.Lock()
	defer c.Unlock()
	if c.hasClosed {
		return
	}
	if c.trackQuitChan != nil {
		close(c.trackQuitChan)
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
	c.Log("write message %s\n", string(j))
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
				fmt.Printf("WriteRTCP err '%s'\n", err)
				return
			}
		}
	}()

	// Create a local track, all our SFU clients will be fed via this track
	localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "audio", "babelcast")
	if newTrackErr != nil {
		panic(newTrackErr)
	}
	c.Log("trackhandler sending localtrack\n")
	c.peer.localTrackChan <- localTrack
	c.Log("trackhandler sent localtrack\n")

	rtpBuf := make([]byte, 1400)
	for {
		i, _, readErr := remoteTrack.Read(rtpBuf)
		if readErr != nil {
			fmt.Printf("remoteTrack.Read err '%s'\n", readErr)
			return
		}
		//		fmt.Printf("remoteTrack.Read len %d bytes\n", i)

		// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
		_, err := localTrack.Write(rtpBuf[:i])
		if err != nil {
			fmt.Printf("localTrack.write err '%s'\n", err)
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
		c.Log("ice connected\n")
		c.Log("remote SDP\n%s\n", c.peer.pc.RemoteDescription().SDP)
		c.Log("local SDP\n%s\n", c.peer.pc.LocalDescription().SDP)
		c.infoChan <- "ice connected"

	case webrtc.ICEConnectionStateDisconnected:
		c.Log("ice disconnected\n")
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
		c.Log("marshal err %s\n", err)
		return
	}

	m := wsMsg{Key: "ice_candidate", Value: j}
	err = c.writeMsg(m)
	if err != nil {
		c.Log("writemsg err %s\n", err)
		return
	}
}

func (c *Conn) LogHandler(ctx context.Context) {
	defer c.Log("log goroutine quitting...\n")
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-c.errChan:
			j, err := json.Marshal(err.Error())
			if err != nil {
				c.Log("marshal err %s\n", err)
			}
			m := wsMsg{Key: "error", Value: j}
			err = c.writeMsg(m)
			if err != nil {
				c.Log("writemsg err %s\n", err)
			}
			// end the WS session on error
			c.Close()
		case info := <-c.infoChan:
			j, err := json.Marshal(info)
			if err != nil {
				c.Log("marshal err %s\n", err)
			}
			m := wsMsg{Key: "info", Value: j}
			err = c.writeMsg(m)
			if err != nil {
				c.Log("writemsg err %s\n", err)
			}
		}
	}
}

func (c *Conn) PingHandler(ctx context.Context) {
	defer c.Log("ws ping goroutine quitting...\n")
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
				c.Log("ping client, err %s\n", err)
				return
			}
			c.Unlock()
		}
	}
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
