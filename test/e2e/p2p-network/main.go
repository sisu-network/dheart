package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/utils"
)

type SimpleListener struct {
	dataChan chan *p2p.P2PMessage
}

func NewSimpleListener(dataChan chan *p2p.P2PMessage) *SimpleListener {
	return &SimpleListener{
		dataChan: dataChan,
	}
}

func (listener *SimpleListener) OnNetworkMessage(message *p2p.P2PMessage) {
	utils.LogVerbose("There is a new message from", message.FromPeerId)
	utils.LogVerbose(string(message.Data))
}

func initialize() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
}

// Runs this program on 2 different terminal with different index value.
func main() {
	var index, n int
	flag.IntVar(&index, "index", 0, "listening port")
	flag.Parse()

	n = 2

	config, privateKey := p2p.GetMockConnectionConfig(n, index)
	cm := p2p.NewConnectionManager(config)

	dataChan := make(chan *p2p.P2PMessage)

	cm.AddListener(p2p.TSSProtocolID, NewSimpleListener(dataChan))

	err := cm.Start(privateKey)
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 4)

	go func() {
		peerIds := p2p.GetMockPeers(n)
		// Send a message to peers
		for i := range peerIds {
			if i == index {
				continue
			}

			utils.LogVerbose("Sending a message to peer", peerIds[i])

			err = cm.WriteToStream(peerIds[i], p2p.TSSProtocolID, []byte(fmt.Sprintf("Hello from index %d", index)))
			if err != nil {
				panic(err)
			}
		}
	}()

	select {
	case msg := <-dataChan:
		utils.LogVerbose("Message =", string(msg.Data))
	}
}
