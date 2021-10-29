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

// 	keys := make([]string, 15)
// 	for _ = range keys {
// 		privKey := p2p.GeneratePrivateKey("ed25519")
// 		fmt.Printf(`"%s",
// `, hex.EncodeToString(privKey.Bytes()))
	// }
}
