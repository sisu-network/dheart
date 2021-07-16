package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/joho/godotenv"
	"github.com/sisu-network/tuktuk/client"
	"github.com/sisu-network/tuktuk/core"
	"github.com/sisu-network/tuktuk/server"
)

func initialize() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
}

func getSisuClient() *client.Client {
	url := os.Getenv("SISU_SERVER_URL")
	c := client.NewClient(url)
	return c
}

func setupApiServer() {
	c := getSisuClient()

	tuktuk := core.NewTutTuk()

	handler := rpc.NewServer()
	if os.Getenv("USE_ON_MEMORY") == "" {
		handler.RegisterName("tss", server.NewTssApi(tuktuk))
	} else {
		api := server.NewSingleNodeApi(tuktuk, c)
		api.Init()

		handler.RegisterName("tss", api)
	}

	s := server.NewServer(handler, "localhost", 5678)

	go c.TryDial()
	go s.Run()
}

func main() {
	initialize()
	setupApiServer()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
