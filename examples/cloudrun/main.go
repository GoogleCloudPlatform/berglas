// Copyright 2019 The Berglas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/GoogleCloudPlatform/berglas/auto"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", F)

	s := &http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: mux,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	signalCh := make(chan os.Signal)
	signal.Notify(signalCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
	)

	<-signalCh
	log.Println("Shutdown called....")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}

func F(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "API_KEY=%s\n", os.Getenv("API_KEY"))
	fmt.Fprintf(w, "TLS_KEY=%s\n", os.Getenv("TLS_KEY"))
	fmt.Fprintf(w, "\n")

	b, err := ioutil.ReadFile(os.Getenv("TLS_KEY"))
	if err != nil {
		fmt.Fprintf(w, "err reading file contents: %s\n", err)
	} else {
		fmt.Fprintf(w, "file contents: %s\n", b)
	}
}
