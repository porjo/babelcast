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
	"os"
	"time"
)

const httpTimeout = 15 * time.Second

var publisherPassword = ""

var reg *Registry

func main() {
	webRoot := flag.String("webRoot", "html", "web root directory")
	port := flag.Int("port", 8080, "listen on this port")
	flag.Parse()

	log.Printf("Starting server...\n")
	log.Printf("Set web root: %s\n", *webRoot)

	publisherPassword = os.Getenv("PUBLISHER_PASSWORD")
	if publisherPassword != "" {
		log.Printf("Publisher password set\n")
	}

	http.HandleFunc("/ws", wsHandler)
	http.Handle("/", http.FileServer(http.Dir(http.Dir(*webRoot))))

	log.Printf("Listening on port :%d\n", *port)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		WriteTimeout: httpTimeout,
		ReadTimeout:  httpTimeout,
	}

	reg = NewRegistry()

	log.Fatal(srv.ListenAndServe())
}
