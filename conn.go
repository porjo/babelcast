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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/inproc"
)

// channel name should NOT match the negation of valid characters
var channelRegexp = regexp.MustCompile("[^a-zA-Z0-9 ]+")

type Conn struct {
	sync.Mutex

	rtcPeer *WebRTCPeer
	wsConn  *websocket.Conn
	spSock  mangos.Socket

	channelName string
	// store channel name as 4 byte hash
	// used as our pub/sub topic
	spTopic []byte

	errChan       chan error
	infoChan      chan string
	trackQuitChan chan struct{}

	logger *log.Logger

	rtpLastTimestamp uint32

	isPublisher bool
	hasClosed   bool
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

func (c *Conn) setupSession(ctx context.Context, cmd CmdSession) error {
	var err error

	offer := cmd.SessionDescription
	c.rtcPeer, err = NewPC(offer, c.rtcStateChangeHandler, c.rtcTrackHandler)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := c.rtcPeer.pc.CreateAnswer(nil)
	if err != nil {
		return err
	}

	j, err := json.Marshal(answer.Sdp)
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

	if c.rtcPeer == nil {
		return fmt.Errorf("webrtc session not established")
	}

	if cmd.Channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}

	if channelRegexp.MatchString(cmd.Channel) {
		return fmt.Errorf("channel name must contain only alphanumeric characters")
	}
	c.channelName = cmd.Channel

	c.Log("setting up publisher for channel '%s'\n", c.channelName)

	c.setTopic(c.channelName)

	c.Lock()
	c.spSock = pubSocket
	c.Unlock()

	if err := reg.AddPublisher(c.channelName); err != nil {
		return err
	}

	return nil
}

func (c *Conn) connectSubscriber(ctx context.Context, cmd CmdConnect) error {
	var err error

	if c.rtcPeer == nil {
		return fmt.Errorf("webrtc session not established")
	}

	//var username string

	if cmd.Channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}
	if channelRegexp.MatchString(cmd.Channel) {
		return fmt.Errorf("channel name must contain only alphanumeric characters")
	}

	c.channelName = cmd.Channel

	c.Log("setting up subscriber for channel '%s'\n", c.channelName)
	c.Lock()
	if c.spSock, err = sub.NewSocket(); err != nil {
		c.Unlock()
		return fmt.Errorf("can't get new sub socket: %s", err)
	}
	c.Unlock()
	c.spSock.AddTransport(inproc.NewTransport())
	if err = c.spSock.Dial("inproc://babelcast/"); err != nil {
		return fmt.Errorf("sub can't dial %s", err)
	}

	c.setTopic(c.channelName)
	c.Lock()
	if err = c.spSock.SetOption(mangos.OptionSubscribe, c.spTopic); err != nil {
		c.Unlock()
		return fmt.Errorf("sub can't subscribe %s", err)
	}
	c.Unlock()

	if err = reg.AddSubscriber(c.channelName); err != nil {
		return err
	}

	go func() {
		defer c.Log("sub read goroutine quitting...\n")

		var data []byte
		for {

			select {
			case <-ctx.Done():
				c.Close()
				return
			default:
			}
			if data, err = c.spSock.Recv(); err != nil {
				c.errChan <- fmt.Errorf("sub sock recv err %s\n", err)
				continue
			}

			rawPacket := data[4:]
			packet := &rtp.Packet{}
			if err = packet.Unmarshal(rawPacket); err != nil {
				c.errChan <- fmt.Errorf("packet unmarshal err %s\n", err)
				continue
			}

			c.Lock()
			samples := packet.Timestamp - c.rtpLastTimestamp
			c.rtpLastTimestamp = packet.Timestamp
			c.Unlock()

			s := webrtc.RTCSample{Data: packet.Payload, Samples: samples}
			c.rtcPeer.track.Samples <- s
		}
	}()

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
	if c.rtcPeer != nil {
		c.rtcPeer.Close()
	}
	if c.spSock != nil && !c.isPublisher {
		c.spSock.Close()
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
func (c *Conn) rtcTrackHandler(track *webrtc.RTCTrack) {
	go func() {
		var err error
		defer c.Log("rtcTrackhandler goroutine quitting...\n")
		for {
			select {
			case <-c.trackQuitChan:
				return
			case p := <-track.Packets:
				c.Lock()
				if c.spSock == nil {
					// subscriber requested non-existent subscription spTopic
					// this isn't an error as publisher may yet appear for that spTopic
					c.Unlock()
					continue
				}
				// mangoes socket requires []byte where leading bytes is the subscription spTopic
				data := make([]byte, len(c.spTopic))
				copy(data, c.spTopic)
				packet, _ := p.Marshal()
				data = append(data, packet...)
				if err = c.spSock.Send(data); err != nil {
					c.errChan <- fmt.Errorf("pub send failed: %s", err)
				}
				c.Unlock()
			}
		}
	}()
}

// WebRTC callback function
func (c *Conn) rtcStateChangeHandler(connectionState ice.ConnectionState) {

	//var err error

	switch connectionState {
	case ice.ConnectionStateConnected:
		c.Log("ice connected\n")
		c.Log("remote SDP\n%s\n", c.rtcPeer.pc.RemoteDescription().Sdp)
		c.Log("local SDP\n%s\n", c.rtcPeer.pc.LocalDescription().Sdp)
		c.infoChan <- "ice connected"

	case ice.ConnectionStateDisconnected:
		c.Log("ice disconnected\n")
		c.Close()

		// non blocking channel write, as receiving goroutine may already have quit
		select {
		case c.infoChan <- "ice disconnected":
		default:
		}
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

// store a 32bit hash of the channel name in a 4 byte slice
func (c *Conn) setTopic(channelName string) {
	c.Lock()
	c.spTopic = make([]byte, 4)
	binary.LittleEndian.PutUint32(c.spTopic, hash(c.channelName))
	c.Unlock()
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
