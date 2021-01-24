package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/tcteo/bhttp"
)

func echoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "echo: %v\n", vars["echo"])
}

func main() {
	h, err := bhttp.NewBHttp(&bhttp.BHttpOptions{HttpPort: 9000, PromHttpPort: 9001})
	if err != nil {
		log.Fatalf("error creating BHttp: %v", err)
	}
	// h.Mux is a gorilla/mux.Router
	// For demo, set up a / route that serves the pwd index.
	h.Mux.Handle("/", http.FileServer(http.Dir(".")))
	h.Mux.HandleFunc("/echo/{echo}", echoHandler)
	h.Start()

	for {
		time.Sleep(1 * time.Second)
	}

}
