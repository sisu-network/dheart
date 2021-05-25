package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sisu-network/tuktuk/rpc"

	"github.com/sisu-network/tuktuk/core"
	"github.com/sisu-network/tuktuk/server"
)

func setupApiServer() {
	tuktuk := core.NewTutTuk()

	handler := rpc.NewServer(time.Second * 10)
	handler.RegisterName("tss", server.NewTssApi(tuktuk))

	s := server.NewServer(handler, "localhost", 5678)

	go s.Run()
}

func main() {
	setupApiServer()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
