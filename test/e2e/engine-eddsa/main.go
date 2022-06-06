package main

import (
	"flag"
	"fmt"
	"math/big"
	"time"

	"github.com/decred/dcrd/dcrec/edwards/v2"

	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/test/e2e/helper"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
	wtypes "github.com/sisu-network/dheart/worker/types"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

func doKeysign(pids tss.SortedPartyIDs, index int, engine core.Engine, message []byte,
	keygenData *edkeygen.LocalPartySaveData) {
	workId := "keysign0"
	threshold := utils.GetThreshold(len(pids))
	request := wtypes.NewEdSigningRequest(workId, pids, threshold, [][]byte{message},
		[]string{"eth"}, keygenData, 1)

	err := engine.AddRequest(request)
	if err != nil {
		panic(err)
	}
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

	pids := worker.GetTestPartyIds(n)
	nodes := make([]*core.Node, n)

	// Add nodes
	privKeys := p2p.GetAllSecp256k1PrivateKeys(n)

	for i := 0; i < n; i++ {
		pubKey := privKeys[i].PubKey()
		node := core.NewNode(pubKey)
		nodes[i] = node
		pids[i] = node.PartyId
	}

	// Create new engine
	keysignch := make(chan *types.KeysignResult, 1)
	cb := &core.MockEngineCallback{
		OnWorkSigningFinishedFunc: func(request *wtypes.WorkRequest, result *types.KeysignResult) {
			fmt.Println("AAAA")
			keysignch <- result
			fmt.Println("BBBBB")
		},
	}

	database := helper.GetInMemoryDb(index)
	engine := core.NewEngine(nodes[index], cm, database, cb, privKeys[index], config.NewDefaultTimeoutConfig())
	cm.AddListener(p2p.TSSProtocolID, engine)

	// Add nodes
	for i := 0; i < n; i++ {
		engine.AddNodes(nodes)
	}

	time.Sleep(time.Second * 3)

	myKeygen := worker.LoadEdKeygenSavedData(pids)[index]

	pubkey := edwards.NewPublicKey(myKeygen.EDDSAPub.X(), myKeygen.EDDSAPub.Y())
	address := utils.GetAddressFromCardanoPubkey(pubkey.Serialize()).String()
	fmt.Println("keygenResult = ", address)

	message := []byte("this is a test")
	doKeysign(pids, index, engine, message, myKeygen)

	select {
	case result := <-keysignch:
		if result.Outcome != types.OutcomeSuccess {
			panic("signing failed")
		}

		if !edwards.Verify(pubkey, message, new(big.Int).SetBytes(result.Signatures[0][:32]),
			new(big.Int).SetBytes(result.Signatures[0][32:])) {
			panic("Signature failure")
		}
	}
}
