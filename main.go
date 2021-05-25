package main

import (
	"time"

	"github.com/sisu-network/tuktuk/rpc"

	"github.com/sisu-network/tuktuk/server"
)

func main() {
	handler := rpc.NewServer(time.Second * 10)
	handler.RegisterName("tss", &server.TssApi{})

	s := server.NewServer(handler, "localhost", 5678)

	s.Run()
}
