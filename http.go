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
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

const PingInterval = 10 * time.Second
const WriteWait = 10 * time.Second

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type wsMsg struct {
	Key   string
	Value json.RawMessage
}

type CmdConnect struct {
	Channel  string
	Password string
}

func wsHandler(w http.ResponseWriter, r *http.Request) {

	gconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	clientAddress := gconn.RemoteAddr().String()
	xFwdIP := r.Header.Get("X-Forwarded-For")
	if xFwdIP != "" {
		clientAddress += " (" + xFwdIP + ")"
	}

	c := NewConn(gconn)
	defer c.Close()
	c.peer, err = NewWebRTCPeer()
	if err != nil {
		c.logger.Error("NewWebRTCPeer error", "err", err.Error())
		return
	}

	c.logger.Info("client connected", "addr", clientAddress)

	// setup ping/pong to keep connection open
	pingCh := time.Tick(PingInterval)

	// websocket connections support one concurrent reader and one concurrent writer.
	// we put reads in a new goroutine below and leave writes in the main goroutine
	wsInMsg := make(chan wsMsg)
	wsReadQuitChan := make(chan struct{})

	go func() {
		defer close(wsReadQuitChan)
		defer c.logger.Debug("ws read goroutine quit")
		for {
			msgType, raw, err := c.wsConn.ReadMessage()
			if err != nil {
				c.logger.Error("ReadMessage error", "err", err)
				return
			}
			c.logger.Debug("read message", "msg", string(raw))
			if msgType != websocket.TextMessage {
				c.logger.Error("unknown message type", "type", msgType)
				return
			}
			var msg wsMsg
			err = json.Unmarshal(raw, &msg)
			if err != nil {
				c.logger.Error(err.Error())
				return
			}
			wsInMsg <- msg
		}
	}()

	for {
		select {
		case msg := <-wsInMsg:
			err = c.handleWSMsg(msg)
			if err != nil {
				j, _ := json.Marshal(err.Error())
				m := wsMsg{Key: "error", Value: j}
				err = c.writeMsg(m)
				if err != nil {
					c.logger.Error("writemsg error", "err", err.Error())
				}
				return
			}
		case <-wsReadQuitChan:
			return
		case <-c.quitchan:
			c.logger.Debug("quitChan closed")
			return
		case info := <-c.infoChan:
			j, err := json.Marshal(info)
			if err != nil {
				c.logger.Error("marshal error", "err", err.Error())
				return
			}
			m := wsMsg{Key: "info", Value: j}
			err = c.writeMsg(m)
			if err != nil {
				c.logger.Error("writemsg error", "err", err.Error())
				return
			}
		case <-pingCh:
			err := c.wsConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(WriteWait))
			if err != nil {
				c.logger.Error("ping client error", "err", err.Error())
				return
			}
		}
	}
}

func (c *Conn) handleWSMsg(msg wsMsg) error {
	var err error
	switch msg.Key {
	case "ice_candidate":
		var candidate webrtc.ICECandidateInit
		err = json.Unmarshal(msg.Value, &candidate)
		if err != nil {
			return err
		}

		if candidate.Candidate != "" {
			if err = c.peer.pc.AddICECandidate(candidate); err != nil {
				c.logger.Error("AddICECandidate error", "err", err.Error())
				return err
			}
		}
	case "get_channels":
		// send list of channels to client
		channels := reg.GetChannels()
		c.logger.Debug("channels", "c", channels)
		j, err := json.Marshal(channels)
		if err != nil {
			c.logger.Error("getchannels marshal", "err", err)
			return err
		}
		m := wsMsg{Key: "channels", Value: j}
		err = c.writeMsg(m)
		if err != nil {
			c.logger.Error(err.Error())
			return err
		}
	case "session_subscriber":
		// subscriber session is only partially setup here as we have to wait for
		// channel selection to complete the setup
		var offer webrtc.SessionDescription
		err = json.Unmarshal(msg.Value, &offer)
		if err != nil {
			return err
		}
		if err := c.peer.pc.SetRemoteDescription(offer); err != nil {
			return err
		}
		// If there is no error, send a success message
		m := wsMsg{Key: "session_received"}
		err = c.writeMsg(m)
		if err != nil {
			c.logger.Error(err.Error())
			return err
		}
	case "session_publisher":
		c.isPublisher = true
		var offer webrtc.SessionDescription
		err = json.Unmarshal(msg.Value, &offer)
		if err != nil {
			return err
		}

		err = c.setupSessionPublisher(offer)
		if err != nil {
			c.logger.Error("setupSession error", "err", err)
			return err
		}
		if publisherPassword != "" {
			m := wsMsg{Key: "password_required"}
			err = c.writeMsg(m)
			if err != nil {
				c.logger.Error(err.Error())
				return err
			}
		}
	case "connect_publisher":
		cmd := CmdConnect{}
		err = json.Unmarshal(msg.Value, &cmd)
		if err != nil {
			return err
		}
		err := c.connectPublisher(cmd)
		if err != nil {
			c.logger.Error("connectPublisher error", "err", err)
			return err
		}
	case "connect_subscriber":
		cmd := CmdConnect{}
		err = json.Unmarshal(msg.Value, &cmd)
		if err != nil {
			return err
		}

		// finish subscriber session setup here
		c.channelName = cmd.Channel
		err = c.setupSessionSubscriber()
		if err != nil {
			c.logger.Error("setupSession error", "err", err)
			return err
		}

		if cmd.Channel == "" {
			return fmt.Errorf("channel cannot be empty")
		}
		if channelRegexp.MatchString(cmd.Channel) {
			return fmt.Errorf("channel name must contain only alphanumeric characters")
		}

		c.logger.Info("setting up subscriber for channel", "channel", c.channelName)

		s := reg.NewSubscriber()
		c.clientID = s.ID

		go func() {
			for {
				select {
				case <-c.quitchan:
					return
				case <-s.QuitChan:
					j, _ := json.Marshal(c.channelName)
					m := wsMsg{Key: "channel_closed", Value: j}
					c.writeMsg(m)
					close(c.quitchan)
					return
				}
			}
		}()

		if err := reg.AddSubscriber(c.channelName, s); err != nil {
			return err
		}
	}
	return nil
}
