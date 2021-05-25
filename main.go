package main

import (
	"github.com/sisu-network/dheart/server"
)

func main() {
	s := &server.Server{}

	s.Run("localhost", 5678)
}
