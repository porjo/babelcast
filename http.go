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
	//Username string
	Channel  string
	Password string

	SessionDescription string
}

type CmdSession struct {
	SessionDescription string
	IsSubscriber       bool
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

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	c.Log("client connected, addr %s\n", clientAddress)

	go c.LogHandler(ctx)
	// setup ping/pong to keep connection open
	go c.PingHandler(ctx)

	for {
		msgType, raw, err := c.wsConn.ReadMessage()
		if err != nil {
			c.Log("ReadMessage err %s\n", err)
			break
		}

		c.Log("read message %s\n", string(raw))

		if msgType == websocket.TextMessage {
			var msg wsMsg
			err = json.Unmarshal(raw, &msg)
			if err != nil {
				c.errChan <- err
				continue
			}

			switch msg.Key {
			case "get_channels":
				// send list of channels to client
				channels := reg.GetChannels()
				fmt.Printf("channels %v\n", channels)
				j, err := json.Marshal(channels)
				if err != nil {
					c.Log("getchannels marshal: %s\n", err)
					continue
				}
				m := wsMsg{Key: "channels", Value: j}
				err = c.writeMsg(m)
				if err != nil {
					c.Log("%s\n", err)
					continue
				}
			case "session_publisher":
				cmd := CmdSession{}
				err = json.Unmarshal(msg.Value, &cmd)
				if err != nil {
					c.errChan <- err
					continue
				}

				err = c.setupSessionPublisher(ctx, cmd)
				if err != nil {
					c.Log("setupSession error: %s\n", err)
					c.errChan <- err
					continue
				}
			case "connect_publisher":
				cmd := CmdConnect{}
				err = json.Unmarshal(msg.Value, &cmd)
				if err != nil {
					c.errChan <- err
					continue
				}
				err := c.connectPublisher(ctx, cmd)
				if err != nil {
					c.Log("connectPublisher error: %s\n", err)
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

				if cmd.SessionDescription == "" {
					err := fmt.Errorf("Subscriber sessionDescription was empty")
					c.Log("connectSubscriber error: %s\n", err)
					c.errChan <- err
					continue
				}

				cs := CmdSession{}
				cs.SessionDescription = cmd.SessionDescription
				c.channelName = cmd.Channel

				err = c.setupSessionSubscriber(ctx, cs)
				if err != nil {
					c.Log("setupSession error: %s\n", err)
					c.errChan <- err
					continue
				}

				err := c.connectSubscriber(ctx, cmd)
				if err != nil {
					c.Log("connectSubscriber error: %s\n", err)
					c.errChan <- err
					continue
				}
				defer func() {
					reg.RemoveSubscriber(c.channelName)
				}()
			}

		} else {
			c.Log("unknown message type - close websocket\n")
			break
		}
	}
	// this will trigger all goroutines to quit
	c.Log("end handler\n")
}
