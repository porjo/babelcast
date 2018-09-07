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
)

const httpTimeout = 15 * time.Second

func main() {
	webRootProducer := flag.String("webRootProducer", "htmlProducer", "web root directory for producer")
	webRootConsumer := flag.String("webRootConsumer", "htmlConsumer", "web root directory for consumers")
	port := flag.Int("port", 8080, "listen on this port")
	flag.Parse()

	log.Printf("Starting server...\n")
	log.Printf("Set producer web root: %s\n", *webRootProducer)
	log.Printf("Set consumer web root: %s\n", *webRootConsumer)

	r := mux.NewRouter()

	r.Handle("/websocket", &wsHandler{})

	r.Handle("/producer", http.FileServer(http.Dir(*webRootProducer)))
	r.HandleFunc("/producer/{channel}", ProducerHandler).Methods("PUT")
	r.Handle("/consumer", http.FileServer(http.Dir(*webRootConsumer)))
	r.HandleFunc("/consumer/{channel}", ConsumerHandler).Methods("GET")

	log.Printf("Listening on port :%d\n", *port)

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf(":%d", *port),
		WriteTimeout: httpTimeout,
		ReadTimeout:  httpTimeout,
	}

	log.Fatal(srv.ListenAndServe())
}
