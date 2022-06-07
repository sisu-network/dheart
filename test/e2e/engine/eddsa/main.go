package main

import (
	"flag"
	"math/rand"
	"os"
	"time"

	cardanobf "github.com/echovl/cardano-go/blockfrost"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/echovl/cardano-go"
	"github.com/sisu-network/lib/log"

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

func randByteArray(n int, seed int) []byte {
	rand.Seed(int64(seed))

	bz := make([]byte, n)
	for i := 0; i < n; i++ {
		bz[i] = byte(rand.Intn(256))
	}

	return bz
}

func getCardanoNode() cardano.Node {
	projectId := os.Getenv("PROJECT_ID")
	if len(projectId) == 0 {
		panic("project id is empty")
	}

	node := cardanobf.NewNode(cardano.Testnet, projectId)
	return node
}

func getCardanoTransferTransaction(node cardano.Node, sender cardano.Address) *cardano.Tx {
	log.Info("Sender = ", sender)
	receiver, err := cardano.NewAddress("addr_test1vqxyzpun2fpqafvkxxxceu5r8yh4dccy6xdcynnchd4dr7qtjh44z")
	if err != nil {
		panic(err)
	}

	tx, err := helper.BuildTx(node, cardano.Testnet, sender, receiver, cardano.NewValue(1e6))
	if err != nil {
		panic(err)
	}

	return tx
}

func doKeysign(pids tss.SortedPartyIDs, index int, engine core.Engine, message []byte,
	keygenData *edkeygen.LocalPartySaveData) {
	workId := "keysign0"
	threshold := utils.GetThreshold(len(pids))
	request := wtypes.NewEdSigningRequest(workId, pids, threshold, [][]byte{message},
		[]string{"eth"}, keygenData)

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
			keysignch <- result
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
	address := utils.GetAddressFromCardanoPubkey(pubkey.Serialize())
	log.Info("sender address = ", address)

	node := getCardanoNode()
	tx := getCardanoTransferTransaction(node, address)
	message, err := tx.Hash()
	if err != nil {
		panic(err)
	}

	doKeysign(pids, index, engine, message, myKeygen)

	select {
	case result := <-keysignch:
		if result.Outcome != types.OutcomeSuccess {
			panic("signing failed")
		}

		for i := range tx.WitnessSet.VKeyWitnessSet {
			tx.WitnessSet.VKeyWitnessSet[i] = cardano.VKeyWitness{VKey: pubkey.Serialize(), Signature: result.Signatures[0]}
		}
	}

	// Submit the tx here.
	if index == 0 {
		hash, err := node.SubmitTx(tx)
		if err != nil {
			panic(err)
		}
		log.Info("Hash = ", hash)
	}
}
