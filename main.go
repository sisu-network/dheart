package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/joho/godotenv"
	"github.com/sisu-network/tuktuk/core"
	"github.com/sisu-network/tuktuk/server"
)

func initialize() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
}

func setupApiServer() {
	tuktuk := core.NewTutTuk()

	handler := rpc.NewServer()
	if os.Getenv("USE_ON_MEMORY") == "" {
		handler.RegisterName("tss", server.NewTssApi(tuktuk))
	} else {
		handler.RegisterName("tss", server.NewSingleNodeApi(tuktuk))
	}

	s := server.NewServer(handler, "localhost", 5678)

	go s.Run()
}

func main() {
	initialize()
	setupApiServer()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
