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
	Username           string
	Channel            string
	SessionDescription string
}

func producerHandler(w http.ResponseWriter, r *http.Request) {

	gconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	c := &Conn{}
	c.errChan = make(chan error)
	c.infoChan = make(chan string)
	// wrap Gorilla conn with our conn so we can extend functionality
	c.conn = gconn
	defer c.conn.Close()

	log.Printf("WS %x: producer client connected, addr %s\n", c.conn.RemoteAddr(), c.conn.RemoteAddr())

	go c.LogHandler(ctx)
	// setup ping/pong to keep connection open
	go c.PingHandler(ctx)

	for {
		msgType, raw, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("WS %x: ReadMessage err %s\n", c.conn.RemoteAddr(), err)
			break
		}

		log.Printf("WS %x: read message %s\n", c.conn.RemoteAddr(), string(raw))

		if msgType == websocket.TextMessage {
			var msg wsMsg
			err = json.Unmarshal(raw, &msg)
			if err != nil {
				c.errChan <- err
				continue
			}

			if msg.Key == "connect" {
				cmd := CmdConnect{}
				err = json.Unmarshal(msg.Value, &cmd)
				if err != nil {
					c.errChan <- err
					continue
				}
				err := c.connectProducerHandler(ctx, cmd)
				if err != nil {
					log.Printf("connectHandler error: %s\n", err)
					c.errChan <- err
					continue
				}
			}

		} else {
			log.Printf("unknown message type - close websocket\n")
			break
		}
	}
	// this will trigger all goroutines to quit
	ctxCancel()
	log.Printf("WS %x: end handler\n", c.conn.RemoteAddr())
}

func consumerHandler(w http.ResponseWriter, r *http.Request) {

	gconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	c := &Conn{}
	c.errChan = make(chan error)
	c.infoChan = make(chan string)
	// wrap Gorilla conn with our conn so we can extend functionality
	c.conn = gconn
	defer c.conn.Close()

	log.Printf("WS %x: consumer client connected, addr %s\n", c.conn.RemoteAddr(), c.conn.RemoteAddr())

	go c.LogHandler(ctx)
	// setup ping/pong to keep connection open
	go c.PingHandler(ctx)

	for {
		msgType, raw, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("WS %x: ReadMessage err %s\n", c.conn.RemoteAddr(), err)
			break
		}

		log.Printf("WS %x: read message %s\n", c.conn.RemoteAddr(), string(raw))

		if msgType == websocket.TextMessage {
			var msg wsMsg
			err = json.Unmarshal(raw, &msg)
			if err != nil {
				c.errChan <- err
				continue
			}

			if msg.Key == "connect" {
				cmd := CmdConnect{}
				err = json.Unmarshal(msg.Value, &cmd)
				if err != nil {
					c.errChan <- err
					continue
				}
				err := c.connectConsumerHandler(ctx, cmd)
				if err != nil {
					log.Printf("connectHandler error: %s\n", err)
					c.errChan <- err
					continue
				}
			}

		} else {
			log.Printf("unknown message type - close websocket\n")
			break
		}
	}
	// this will trigger all goroutines to quit
	ctxCancel()
	log.Printf("WS %x: end handler\n", c.conn.RemoteAddr())
}
