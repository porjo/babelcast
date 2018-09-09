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
	Username string
	Channel  string
}

type CmdSession struct {
	SessionDescription string
}

func wsHandler(w http.ResponseWriter, r *http.Request) {

	gconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	c := NewConn(gconn)
	defer c.Close()

	c.Log("client connected, addr %s\n", c.wsConn.RemoteAddr())

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
			case "session":
				cmd := CmdSession{}
				err = json.Unmarshal(msg.Value, &cmd)
				if err != nil {
					c.errChan <- err
					continue
				}
				err := c.setupSession(ctx, cmd)
				if err != nil {
					c.Log("setupSession error: %s\n", err)
					c.errChan <- err
					continue
				}
			case "connect_publisher":
				c.isPublisher = true
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
				defer func(c *Conn) {
					reg.Lock()
					reg.Channels[c.channel] = false
					reg.Unlock()
				}(c)
			case "connect_subscriber":
				cmd := CmdConnect{}
				err = json.Unmarshal(msg.Value, &cmd)
				if err != nil {
					c.errChan <- err
					continue
				}
				err := c.connectSubscriber(ctx, cmd)
				if err != nil {
					c.Log("connectSubscriber error: %s\n", err)
					c.errChan <- err
					continue
				}
			}

		} else {
			c.Log("unknown message type - close websocket\n")
			break
		}
	}
	// this will trigger all goroutines to quit
	ctxCancel()
	c.Log("end handler\n")
}
