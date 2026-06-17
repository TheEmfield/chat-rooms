package main

import (
	"fmt"

	"github.com/TheEmfield/chat-rooms/internal/wsserver"
)

const (
	addr = "127.0.0.1:8080"
)

func main() {
	wsSrv := wsserver.NewWsServer(addr)
	fmt.Println("started wsserver")
	if err := wsSrv.Start(); err != nil {
		fmt.Println("error with wsserver: v%", err)
	}
}
