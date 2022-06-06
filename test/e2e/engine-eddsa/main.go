package main

import (
	"flag"
	"fmt"
	"time"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"

	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/test/e2e/helper"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	wtypes "github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/tss"
)

func doKeygen(pids tss.SortedPartyIDs, index int, engine core.Engine, outCh chan *types.KeygenResult) *types.KeygenResult {
	workId := "keygen0"
	threshold := utils.GetThreshold(len(pids))
	request := wtypes.NewEdKeygenRequest(workId, pids, threshold)
	err := engine.AddRequest(request)
	if err != nil {
		panic(err)
	}

	var result *types.KeygenResult
	select {
	case result = <-outCh:
	case <-time.After(time.Second * 100):
		panic("Keygen timeout")
	}

	return result
}

func main() {
	var index, n int
	flag.IntVar(&index, "index", 0, "listening port")
	flag.IntVar(&n, "n", 2, "number of nodes in the test")
	flag.Parse()

	cfg, privateKey := p2p.GetMockSecp256k1Config(n, index)
	cm := p2p.NewConnectionManager(cfg)
	err := cm.Start(privateKey, "secp256k1")
	if err != nil {
		panic(err)
	}

	pids := make([]*tss.PartyID, n)
	allKeys := p2p.GetAllSecp256k1PrivateKeys(n)
	nodes := make([]*core.Node, n)
	tendermintPubKeys := make([]ctypes.PubKey, n)

	// Add nodes
	privKeys := p2p.GetAllSecp256k1PrivateKeys(n)
	for i := 0; i < n; i++ {
		pubKey := privKeys[i].PubKey()
		node := core.NewNode(pubKey)
		nodes[i] = node
		pids[i] = node.PartyId
	}
	pids = tss.SortPartyIDs(pids)

	// Create new engine
	keygenCh := make(chan *types.KeygenResult)
	keysignch := make(chan *types.KeysignResult)
	cb := &core.MockEngineCallback{
		OnWorkKeygenFinishedFunc: func(result *types.KeygenResult) {
			keygenCh <- result
		},
		OnWorkSigningFinishedFunc: func(request *wtypes.WorkRequest, result *types.KeysignResult) {
			keysignch <- result
		},
	}

	database := helper.GetInMemoryDb(index)
	engine := core.NewEngine(nodes[index], cm, database, cb, allKeys[index], config.NewDefaultTimeoutConfig())
	cm.AddListener(p2p.TSSProtocolID, engine)

	// Add nodes
	for i := 0; i < n; i++ {
		engine.AddNodes(nodes)
		tendermintPubKeys[i] = privKeys[i].PubKey()
	}

	time.Sleep(time.Second * 3)

	// Run keygen
	keygenResult := doKeygen(pids, index, engine, keygenCh)

	fmt.Println("keygenResult = ", keygenResult)
}
