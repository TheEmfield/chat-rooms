package main

import (
	"fmt"

	"github.com/TheEmfield/chat-rooms/internal/wsserver"
)

const (
	addr = ":8080"
)

func main() {
	wsSrv := wsserver.NewWsServer(addr)
	fmt.Println("started wsserver")
	if err := wsSrv.Start(); err != nil {
		fmt.Printf("error with wsserver: %v", err)
	}
	wsSrv.Stop()
}
