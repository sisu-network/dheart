package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sisu-network/dheart/run"
)

func main() {
	run.Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
