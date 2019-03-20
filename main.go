package main

import (
	"flag"
	"log"
	"time"
	"chess/server"
)

func main() {
	var address = flag.String("address", ":8080", "TCPServer")
	var timeout = flag.Duration("timeout", time.Minute*5, "ReadTimeoutForClients")
	flag.Parse()
	server, err := server.NewServerAndListen(*address, *timeout)
	if err != nil {
		log.Fatal("Unable to create TCPServer for address={}", *address)
	}
	defer server.Close()
	log.Printf("Successfully connected to address=%v\n", *address)
	<-server.ErrCh
}
