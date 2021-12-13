package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/lib/log"
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
	log.Verbose("There is a new message from", message.FromPeerId)
	log.Verbose(string(message.Data))
	listener.dataChan <- message
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
	flag.IntVar(&index, "index", 0, "index of the node")
	flag.Parse()

	n = 2

	config, privateKey := p2p.GetMockSecp256k1Config(n, index)
	cm := p2p.NewConnectionManager(config)

	dataChan := make(chan *p2p.P2PMessage)

	cm.AddListener(p2p.TSSProtocolID, NewSimpleListener(dataChan))

	err := cm.Start(privateKey, "secp256k1")
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 8)

	go func() {
		peerIds := p2p.GetMockPeers(n, "secp256k1")
		// Send a message to peers
		for i := range peerIds {
			if i == index {
				continue
			}

			log.Verbose("Sending a message to peer", peerIds[i])

			err = cm.WriteToStream(peerIds[i], p2p.TSSProtocolID, []byte(fmt.Sprintf("Hello from index %d", index)))
			if err != nil {
				panic(err)
			}
		}
	}()

	select {
	case <-time.After(time.Second * 10):
		panic("Timeout!")
	case msg := <-dataChan:
		log.Verbose("Message = ", string(msg.Data))
		log.Info("Test passed!")
	}
}
