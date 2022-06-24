package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	libchain "github.com/sisu-network/lib/chain"

	cardanobf "github.com/echovl/cardano-go/blockfrost"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/echovl/cardano-go"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/sisu-network/lib/log"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	mock "github.com/sisu-network/dheart/test/e2e/app/sisu-mock"
	"github.com/sisu-network/dheart/test/e2e/helper"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
)

type MockSisuNode struct {
	server  *mock.Server
	client  *mock.DheartClient
	privKey ctypes.PrivKey
}

func createNodes(index int, n int, keygenCh chan *types.KeygenResult, keysignCh chan *types.KeysignResult, pingCh chan string) *MockSisuNode {
	port := 25456 + index
	heartPort := 5678 + index

	handler := ethRpc.NewServer()
	handler.RegisterName("tss", mock.NewApi(keygenCh, keysignCh, pingCh))

	s := mock.NewServer(handler, "0.0.0.0", uint16(port))

	client, err := mock.DialDheart(fmt.Sprintf("http://0.0.0.0:%d", heartPort))
	if err != nil {
		panic(err)
	}

	privKey := p2p.GetAllSecp256k1PrivateKeys(n)[index]

	return &MockSisuNode{
		server:  s,
		client:  client,
		privKey: privKey,
	}
}

func bootstrapNetwork(nodes []*MockSisuNode) {
	n := len(nodes)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	aesKey, err := hex.DecodeString(os.Getenv("AES_KEY_HEX"))
	if err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		go func(index int) {
			encrypt, err := utils.AESDEncrypt(nodes[index].privKey.Bytes(), []byte(aesKey))
			if err != nil {
				panic(err)
			}

			nodes[index].client.Ping("sisu")
			nodes[index].client.SetPrivKey(hex.EncodeToString(encrypt), nodes[index].privKey.Type())
			nodes[index].client.SetSisuReady(true)

			wg.Done()
		}(i)
	}

	wg.Wait()
	log.Info("Done Setting private key!")
	time.Sleep(3 * time.Second)
}

func waitForDheartPings(pingChs []chan string) {
	log.Info("waiting for all dheart instances to ping")

	wg := &sync.WaitGroup{}
	wg.Add(len(pingChs))
	for _, ch := range pingChs {
		go func(pingCh chan string) {
			<-pingCh
			wg.Done()
		}(ch)
	}

	wg.Wait()
	log.Info("Received all ping from all dheart instances")
}

func insertKeygenData(n, index int) {
	cfg, err := config.ReadConfig(filepath.Join(fmt.Sprintf("./nodes/node%d", index), "dheart.toml"))
	if err != nil {
		panic(err)
	}

	database := db.NewDatabase(&cfg.Db)
	err = database.Init()
	if err != nil {
		panic(err)
	}

	pids := worker.GetTestPartyIds(n)
	keygenOutput := worker.LoadEdKeygenSavedData(pids)[index]
	err = database.SaveEdKeygen(libchain.KEY_TYPE_EDDSA, "keygen0", pids, keygenOutput)
	if err != nil {
		panic(err)
	}
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

func doKeysign(nodes []*MockSisuNode, tendermintPubKeys []ctypes.PubKey,
	keysignChs []chan *types.KeysignResult, publicKeyBytes []byte, message []byte) []byte {
	n := len(nodes)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	for i := 0; i < len(nodes); i++ {
		request := &types.KeysignRequest{
			KeyType: libchain.KEY_TYPE_EDDSA,
			KeysignMessages: []*types.KeysignMessage{
				{
					Id:          "Keysign0",
					OutChain:    "cardano-testnet",
					OutHash:     hex.EncodeToString(message),
					BytesToSign: message,
				},
			},
		}

		nodes[i].client.KeySign(request, tendermintPubKeys)
	}

	results := make([]*types.KeysignResult, n)
	for i := 0; i < n; i++ {
		go func(index int) {
			result := <-keysignChs[index]
			results[index] = result
			wg.Done()
		}(i)
	}

	wg.Wait()

	for i := 0; i < n; i++ {
		if bytes.Compare(results[i].Signatures[0], results[0].Signatures[0]) != 0 {
			panic("Signature does not match")
		}
	}

	return results[0].Signatures[0]
}

func submitTx(node cardano.Node, tx *cardano.Tx, pubkey *edwards.PublicKey, signature []byte) {
	for i := range tx.WitnessSet.VKeyWitnessSet {
		tx.WitnessSet.VKeyWitnessSet[i] = cardano.VKeyWitness{VKey: pubkey.Serialize(), Signature: signature}
	}

	hash, err := node.SubmitTx(tx)
	if err != nil {
		panic(err)
	}
	log.Info("Hash = ", hash)
}

func main() {
	var n int
	flag.IntVar(&n, "n", 0, "Total nodes")
	flag.Parse()

	if n == 0 {
		n = 2
	}

	helper.LoadConfigEnv("../../../../.env")
	for i := 0; i < n; i++ {
		helper.ResetDb(i)
	}
	// Save mock keygen into db
	for i := 0; i < n; i++ {
		insertKeygenData(n, i)
	}

	keygenChs := make([]chan *types.KeygenResult, n)
	keysignChs := make([]chan *types.KeysignResult, n)
	pingChs := make([]chan string, n)
	tendermintPubKeys := make([]ctypes.PubKey, n)
	nodes := make([]*MockSisuNode, n)

	for i := 0; i < n; i++ {
		keygenChs[i] = make(chan *types.KeygenResult)
		keysignChs[i] = make(chan *types.KeysignResult)
		pingChs[i] = make(chan string)

		nodes[i] = createNodes(i, n, keygenChs[i], keysignChs[i], pingChs[i])
		go nodes[i].server.Run()

		tendermintPubKeys[i] = nodes[i].privKey.PubKey()
	}

	// Waits for all the dheart to send ping messages
	waitForDheartPings(pingChs)

	// Set private keys
	bootstrapNetwork(nodes)

	pids := worker.GetTestPartyIds(2)
	myKeygen := worker.LoadEdKeygenSavedData(pids)[0]

	pubkey := edwards.NewPublicKey(myKeygen.EDDSAPub.X(), myKeygen.EDDSAPub.Y())
	address := utils.GetAddressFromCardanoPubkey(pubkey.Serialize())
	log.Info("sender address = ", address)

	node := getCardanoNode()
	tx := getCardanoTransferTransaction(node, address)
	message, err := tx.Hash()
	if err != nil {
		panic(err)
	}

	signature := doKeysign(nodes, tendermintPubKeys, keysignChs, pubkey.Serialize(), message)
	submitTx(node, tx, pubkey, signature)
}
