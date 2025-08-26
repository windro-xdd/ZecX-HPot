package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	defer conn.Close()
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("ws read: %v", err)
			return
		}
		log.Printf("ws recv: %v", msg)
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("http %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
	w.WriteHeader(200)
	w.Write([]byte("ok"))
}

func main() {
	addr := flag.String("addr", ":8088", "listen address")
	flag.Parse()
	http.HandleFunc("/wss", wsHandler)
	http.HandleFunc("/hook", httpHandler)
	srv := &http.Server{Addr: *addr, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	fmt.Printf("Test collector listening on %s\n", *addr)
	log.Fatal(srv.ListenAndServe())
}
