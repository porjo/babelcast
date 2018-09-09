// Babelcast a WebRTC audio broadcast server

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
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pub"
	"nanomsg.org/go-mangos/transport/inproc"
)

const httpTimeout = 15 * time.Second

var pubSocket mangos.Socket

var reg Registry

func main() {
	webRootPublisher := flag.String("webRootPublisher", "htmlPublisher", "web root directory for publisher")
	webRootSubscriber := flag.String("webRootSubscriber", "htmlSubscriber", "web root directory for subscribers")
	port := flag.Int("port", 8080, "listen on this port")
	flag.Parse()

	log.Printf("Starting server...\n")
	log.Printf("Set publisher web root: %s\n", *webRootPublisher)
	log.Printf("Set subscriber web root: %s\n", *webRootSubscriber)

	r := mux.NewRouter()

	r.PathPrefix("/static/publisher/").Handler(http.StripPrefix("/static/publisher/", http.FileServer(http.Dir(*webRootPublisher))))
	r.PathPrefix("/static/subscriber/").Handler(http.StripPrefix("/static/subscriber/", http.FileServer(http.Dir(*webRootSubscriber))))
	r.HandleFunc("/ws", wsHandler)

	log.Printf("Listening on port :%d\n", *port)

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf(":%d", *port),
		WriteTimeout: httpTimeout,
		ReadTimeout:  httpTimeout,
	}

	var err error

	if pubSocket, err = pub.NewSocket(); err != nil {
		log.Fatalf("can't get new pub socket: %s", err)
	}
	pubSocket.AddTransport(inproc.NewTransport())
	if err = pubSocket.Listen("inproc://babelcast/"); err != nil {
		log.Fatalf("can't listen on pub socket: %s", err)
	}

	reg.Channels = make(map[string]*Channel)

	log.Fatal(srv.ListenAndServe())
}
