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
	"time"

	"github.com/gorilla/websocket"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/ice"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/inproc"
)

// channel name should NOT match the negation of valid characters
var channelRegexp = regexp.MustCompile("[^a-zA-Z0-9 ]+")

type Conn struct {
	rtcPeer *WebRTCPeer
	wsConn  *websocket.Conn
	mSock   mangos.Socket

	channel string
	// store channel name as 4 byte hash
	channelHash []byte

	errChan       chan error
	infoChan      chan string
	trackQuitChan chan struct{}

	logger *log.Logger

	lastTimestamp uint32

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
	c.channel = cmd.Channel
	reg.Lock()
	if inuse, _ := reg.Channels[c.channel]; inuse {
		return fmt.Errorf("channel '%s' is already in use", c.channel)
	} else {
		reg.Channels[c.channel] = true
	}
	reg.Unlock()

	c.Log("setting up publisher for channel '%s'\n", c.channel)

	// store a 32bit hash of the channel name in a 4 byte slice
	c.channelHash = make([]byte, 4)
	binary.LittleEndian.PutUint32(c.channelHash, hash(c.channel))

	c.mSock = pubSocket

	return nil
}

func (c *Conn) connectSubscriber(ctx context.Context, cmd CmdConnect) error {
	var err error
	var channel string

	if c.rtcPeer == nil {
		return fmt.Errorf("webrtc session not established")
	}

	//var username string

	if cmd.Channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}

	channel = cmd.Channel

	c.Log("setting up subscriber for channel '%s'\n", channel)
	if c.mSock, err = sub.NewSocket(); err != nil {
		return fmt.Errorf("can't get new sub socket: %s", err)
	}
	c.mSock.AddTransport(inproc.NewTransport())
	if err = c.mSock.Dial("inproc://babelcast/"); err != nil {
		return fmt.Errorf("sub can't dial %s", err)
	}

	// store a 32bit hash of the channel name in a 4 byte slice
	c.channelHash = make([]byte, 4)
	binary.LittleEndian.PutUint32(c.channelHash, hash(cmd.Channel))

	if err = c.mSock.SetOption(mangos.OptionSubscribe, c.channelHash); err != nil {
		return fmt.Errorf("sub can't subscribe %s", err)
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
			if data, err = c.mSock.Recv(); err != nil {
				c.errChan <- fmt.Errorf("sub sock recv err %s\n", err)
			}

			sp := &SPMessage{}
			sp.Decode(data)

			s := webrtc.RTCSample{Data: sp.payload, Samples: sp.samples}
			c.rtcPeer.track.Samples <- s
		}
	}()

	return nil
}

func (c *Conn) Close() {
	if c.hasClosed {
		return
	}
	if c.trackQuitChan != nil {
		close(c.trackQuitChan)
	}
	if c.rtcPeer != nil {
		c.rtcPeer.Close()
	}
	if c.mSock != nil && !c.isPublisher {
		c.mSock.Close()
	}
	c.hasClosed = true
}

func (c *Conn) writeMsg(val interface{}) error {
	j, err := json.Marshal(val)
	if err != nil {
		return err
	}
	c.Log("write message %s\n", string(j))
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
				if c.mSock != nil {
					sp := &SPMessage{
						samples:     p.Timestamp - c.lastTimestamp,
						payload:     p.Payload,
						channelHash: c.channelHash,
					}
					if err = c.mSock.Send(sp.Encode()); err != nil {
						c.errChan <- fmt.Errorf("pub send failed: %s", err)
					}
					c.lastTimestamp = p.Timestamp
				}
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
			// WriteControl can be called concurrently
			err := c.wsConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(WriteWait))
			if err != nil {
				c.Log("ping client, err %s\n", err)
				return
			}
		}
	}
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// SPMessage represents a message passed over Mangos socket
type SPMessage struct {
	samples     uint32
	payload     []byte
	channelHash []byte
}

// encode to []byte
func (m *SPMessage) Encode() []byte {
	if m.payload == nil || m.channelHash == nil {
		return nil
	}
	data := make([]byte, len(m.channelHash))
	copy(data, m.channelHash)
	samples := make([]byte, 4)
	binary.LittleEndian.PutUint32(samples, m.samples)
	data = append(data, samples...)
	data = append(data, m.payload...)
	return data
}

// decode from []byte
func (m *SPMessage) Decode(data []byte) {
	if len(data) > 8 {
		m.channelHash = data[:4]
		m.samples = binary.LittleEndian.Uint32(data[4:8])
		m.payload = data[8:]
	} else {
		m.channelHash = make([]byte, 0)
		m.payload = make([]byte, 0)
	}
}
