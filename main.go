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
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const httpTimeout = 15 * time.Second

var (
	publisherPassword = ""

	reg *Registry
)

func main() {
	webRoot := flag.String("webRoot", "html", "web root directory")
	port := flag.Int("port", 8080, "listen on this port")
	debug := flag.Bool("debug", false, "enable debug log")
	flag.Parse()

	var programLevel = new(slog.LevelVar) // Info by default
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel}))
	slog.SetDefault(logger)

	if *debug {
		programLevel.Set(slog.LevelDebug)
	}

	/*
		file, _ := os.Create("./cpu.pprof")
		pprof.StartCPUProfile(file)
		defer pprof.StopCPUProfile()
	*/

	slog.Info("starting server")
	slog.Info("set web root", "dir", *webRoot)

	publisherPassword = os.Getenv("PUBLISHER_PASSWORD")
	if publisherPassword != "" {
		slog.Info("publisher password set")
	}

	http.HandleFunc("/ws", wsHandler)
	http.Handle("/", http.FileServer(http.Dir(http.Dir(*webRoot))))

	slog.Info("listening on port", "port", *port)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		WriteTimeout: httpTimeout,
		ReadTimeout:  httpTimeout,
	}

	reg = NewRegistry()

	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error("error starting server", "err", err)
		}
	}()

	// trap sigterm or interrupt and gracefully shutdown the server
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	signal.Notify(sigChan, syscall.SIGTERM)

	// block until a signal is received
	sig := <-sigChan
	slog.Info("got signal", "sig", sig)
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
}
