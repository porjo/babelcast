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
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/ice"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pub"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/inproc"
)

type Conn struct {
	peer *WebRTCPeer
	conn *websocket.Conn
	sock mangos.Socket

	errChan  chan error
	infoChan chan string
}

func (c *Conn) connectProducerHandler(ctx context.Context, cmd CmdConnect) error {
	var err error

	var channel string
	//var username string

	if cmd.Channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}

	channel = cmd.Channel

	offer := cmd.SessionDescription
	c.peer, err = NewPC(offer, c.rtcStateChangeHandler, c.rtcTrackHandler)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := c.peer.pc.CreateAnswer(nil)
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

	log.Printf("setting up producer for channel '%s'\n", channel)
	if c.sock, err = pub.NewSocket(); err != nil {
		return fmt.Errorf("can't get new pub socket: %s", err)
	}
	c.sock.AddTransport(inproc.NewTransport())
	if err = c.sock.Listen("inproc://babelcast/" + channel); err != nil {
		return fmt.Errorf("can't listen on pub socket: %s", err)
	}

	return nil
}

func (c *Conn) connectConsumerHandler(ctx context.Context, cmd CmdConnect) error {
	var err error

	var channel string
	//var username string

	if cmd.Channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}

	channel = cmd.Channel

	offer := cmd.SessionDescription
	c.peer, err = NewPC(offer, c.rtcStateChangeHandler, nil)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := c.peer.pc.CreateAnswer(nil)
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

	log.Printf("setting up consumer for channel '%s'\n", channel)
	if c.sock, err = sub.NewSocket(); err != nil {
		return fmt.Errorf("can't get new sub socket: %s", err)
	}
	c.sock.AddTransport(inproc.NewTransport())
	if err = c.sock.Dial("inproc://babelcast/" + channel); err != nil {
		return fmt.Errorf("sub can't dial %s", err)
	}

	go func() {
		defer log.Printf("sub read goroutine quitting...\n")

		var packet []byte
		for {

			select {
			case <-ctx.Done():
				c.peer.Close()
				return
			default:
			}
			if packet, err = c.sock.Recv(); err != nil {
				c.errChan <- fmt.Errorf("sub sock recv err %s\n", err)
			}

			// FIXME: where to get samples from?
			s := webrtc.RTCSample{Data: packet, Samples: uint32(len(packet))}
			c.peer.track.Samples <- s
		}
	}()

	return nil
}

func (c *Conn) writeMsg(val interface{}) error {
	j, err := json.Marshal(val)
	if err != nil {
		return err
	}
	log.Printf("WS %x: write message %s\n", c.conn.RemoteAddr(), string(j))
	if err = c.conn.WriteMessage(websocket.TextMessage, j); err != nil {
		return err
	}

	return nil
}

// WebRTC callback function
func (c *Conn) rtcTrackHandler(track *webrtc.RTCTrack) {
	fmt.Printf("rtcTrackHandler %+v\n", track)
	go func() {
		var err error
		defer log.Printf("rtcTrackhandler goroutine quitting...\n")
		for {
			select {
			//		case <-ctx.Done():
			//			c.peer.Close()
			//			return
			case p := <-track.Packets:
				fmt.Printf("peer packet %d\n", len(p.Payload))
				if err = c.sock.Send(p.Payload); err != nil {
					c.errChan <- fmt.Errorf("pub send failed: %s", err)
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
		log.Printf("ice connected\n")
		log.Printf("remote SDP\n%s\n", c.peer.pc.RemoteDescription().Sdp)
		log.Printf("local SDP\n%s\n", c.peer.pc.LocalDescription().Sdp)
		c.infoChan <- "ice connected"

	case ice.ConnectionStateDisconnected:
		log.Printf("ice disconnected\n")

		// non blocking channel write, as receiving goroutine may already have quit
		select {
		case c.infoChan <- "ice disconnected":
		default:
		}
	}
}

func (c *Conn) LogHandler(ctx context.Context) {
	defer log.Printf("log goroutine quitting...\n")
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-c.errChan:
			j, err := json.Marshal(err.Error())
			if err != nil {
				log.Printf("marshal err %s\n", err)
			}
			m := wsMsg{Key: "error", Value: j}
			err = c.writeMsg(m)
			if err != nil {
				log.Printf("writemsg err %s\n", err)
			}
			// end the WS session on error
			c.conn.Close()
		case info := <-c.infoChan:
			j, err := json.Marshal(info)
			if err != nil {
				log.Printf("marshal err %s\n", err)
			}
			m := wsMsg{Key: "info", Value: j}
			err = c.writeMsg(m)
			if err != nil {
				log.Printf("writemsg err %s\n", err)
			}
		}
	}
}

func (c *Conn) PingHandler(ctx context.Context) {
	defer log.Printf("ws ping goroutine quitting...\n")
	pingCh := time.Tick(PingInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-pingCh:
			// WriteControl can be called concurrently
			err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(WriteWait))
			if err != nil {
				log.Printf("WS %x: ping client, err %s\n", c.conn.RemoteAddr(), err)
				return
			}
		}
	}
}
