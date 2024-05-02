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
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
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

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	c.logger.Info("client connected", "addr", clientAddress)

	go c.LogHandler(ctx)
	// setup ping/pong to keep connection open
	go c.PingHandler(ctx)

	for {
		msgType, raw, err := c.wsConn.ReadMessage()
		if err != nil {
			c.logger.Error("ReadMessage error", "err", err)
			break
		}

		c.logger.Debug("read message", "msg", string(raw))

		if msgType != websocket.TextMessage {
			c.logger.Error("unknown message type - close websocket")
			break
		}

		var msg wsMsg
		err = json.Unmarshal(raw, &msg)
		if err != nil {
			c.errChan <- err
			continue
		}

		switch msg.Key {
		case "ice_candidate":
			var candidate webrtc.ICECandidateInit
			err = json.Unmarshal(msg.Value, &candidate)
			if err != nil {
				c.errChan <- err
				continue
			}

			if candidate.Candidate != "" {
				if err = c.peer.pc.AddICECandidate(candidate); err != nil {
					c.logger.Error("AddICECandidate error", "err", err.Error())
					continue
				}
			}
		case "get_channels":
			// send list of channels to client
			channels := reg.GetChannels()
			c.logger.Debug("channels", "c", channels)
			j, err := json.Marshal(channels)
			if err != nil {
				c.logger.Error("getchannels marshal", "err", err)
				continue
			}
			m := wsMsg{Key: "channels", Value: j}
			err = c.writeMsg(m)
			if err != nil {
				c.logger.Error(err.Error())
				continue
			}
		case "session_subscriber":
			// subscriber session is only partially setup here as we have to wait for
			// channel selection to complete the setup
			var offer webrtc.SessionDescription
			err = json.Unmarshal(msg.Value, &offer)
			if err != nil {
				c.errChan <- err
				continue
			}
			if err := c.peer.pc.SetRemoteDescription(offer); err != nil {
				c.errChan <- err
				continue
			}
			// If there is no error, send a success message
			m := wsMsg{Key: "session_received"}
			err = c.writeMsg(m)
			if err != nil {
				c.logger.Error(err.Error())
				continue
			}
		case "session_publisher":
			var offer webrtc.SessionDescription
			err = json.Unmarshal(msg.Value, &offer)
			if err != nil {
				c.errChan <- err
				continue
			}

			err = c.setupSessionPublisher(offer)
			if err != nil {
				c.logger.Error("setupSession error", "err", err)
				c.errChan <- err
				continue
			}
			if publisherPassword != "" {
				m := wsMsg{Key: "password_required"}
				err = c.writeMsg(m)
				if err != nil {
					c.logger.Error(err.Error())
					continue
				}
			}
		case "connect_publisher":
			cmd := CmdConnect{}
			err = json.Unmarshal(msg.Value, &cmd)
			if err != nil {
				c.errChan <- err
				continue
			}
			err := c.connectPublisher(cmd)
			if err != nil {
				c.logger.Error("connectPublisher error", "err", err)
				c.errChan <- err
				continue
			}
			defer func() {
				reg.RemovePublisher(c.channelName)
			}()
		case "connect_subscriber":
			cmd := CmdConnect{}
			err = json.Unmarshal(msg.Value, &cmd)
			if err != nil {
				c.errChan <- err
				continue
			}

			// finish subscriber session setup here
			c.channelName = cmd.Channel
			err = c.setupSessionSubscriber()
			if err != nil {
				c.logger.Error("setupSession error", "err", err)
				c.errChan <- err
				continue
			}

			if cmd.Channel == "" {
				c.errChan <- fmt.Errorf("channel cannot be empty")
				continue
			}
			if channelRegexp.MatchString(cmd.Channel) {
				c.errChan <- fmt.Errorf("channel name must contain only alphanumeric characters")
				continue
			}

			c.logger.Info("setting up subscriber for channel", "channel", c.channelName)

			if err := reg.AddSubscriber(c.channelName); err != nil {
				c.errChan <- err
				continue
			}

			defer func() {
				reg.RemoveSubscriber(c.channelName)
			}()
		}
	}
	c.logger.Info("end WS handler\n")
}
