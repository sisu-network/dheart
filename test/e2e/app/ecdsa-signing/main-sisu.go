package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	libchain "github.com/sisu-network/lib/chain"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/sisu-network/lib/log"

	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	mock "github.com/sisu-network/dheart/test/e2e/app/sisu-mock"
	"github.com/sisu-network/dheart/test/e2e/helper"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
)

const (
	TEST_CHAIN = "ganache1"
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
	keygenOutput := worker.LoadEcKeygenSavedData(pids)[index]
	err = database.SaveEcKeygen(libchain.KEY_TYPE_ECDSA, "keygen0", pids, keygenOutput)
	if err != nil {
		panic(err)
	}
}

func doKeysign(nodes []*MockSisuNode, tendermintPubKeys []ctypes.PubKey, keysignChs []chan *types.KeysignResult,
	publicKeyBytes []byte, chainId *big.Int) []*etypes.Transaction {
	n := len(nodes)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	tx := generateEthTx()
	signer := etypes.NewEIP2930Signer(libchain.GetChainIntFromId(TEST_CHAIN))
	hash := signer.Hash(tx)
	hashBytes := hash[:]

	for i := 0; i < n; i++ {
		request := &types.KeysignRequest{
			KeyType: libchain.KEY_TYPE_ECDSA,
			KeysignMessages: []*types.KeysignMessage{
				{
					Id:          "Keysign0",
					OutChain:    TEST_CHAIN,
					OutHash:     "Hash0",
					BytesToSign: hashBytes,
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

	// Verify signing result.
	// Check that if all signatures are the same.
	for i := 0; i < n; i++ {
		if len(results[i].Signatures) != len(results[0].Signatures) {
			panic("Length of signature arrays do not match")
		}

		for j := 0; j < len(results[0].Signatures); j++ {
			match := bytes.Equal(results[0].Signatures[j], results[i].Signatures[j])
			if !match {
				panic(fmt.Sprintf("Signatures do not match at node %d and job %d", i, j))
			}
		}
	}

	signedTxs := make([]*etypes.Transaction, len(results[0].Signatures))
	for j := 0; j < len(results[0].Signatures); j++ {
		log.Info("Signature after signing = ", results[0].Signatures[j])
		sigPublicKey, err := crypto.Ecrecover(hashBytes, results[0].Signatures[j])
		if err != nil {
			panic(err)
		}

		sigPublicKey = sigPublicKey[1:]

		matches := bytes.Equal(sigPublicKey, publicKeyBytes)
		if !matches {
			log.Error("sigPublicKey = ", hex.EncodeToString(sigPublicKey))
			log.Error("publicKeyBytes = ", hex.EncodeToString(publicKeyBytes))
			panic("Reconstructed pubkey does not match pubkey")
		} else {
			log.Info("Signature matched")
		}

		signedTx, err := tx.WithSignature(signer, results[0].Signatures[j])
		if err != nil {
			panic(err)
		}

		signedTxs[j] = signedTx
	}

	return signedTxs
}

func generateEthTx() *etypes.Transaction {
	nonce := 0

	value := big.NewInt(100000000000000000) // in wei (0.1 eth)
	gasLimit := uint64(8_000_000)           // in units
	gasPrice := big.NewInt(100000000)

	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	var data []byte
	rawTx := etypes.NewTransaction(uint64(nonce), toAddress, value, gasLimit, gasPrice, data)

	return rawTx
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

	pids := worker.GetTestPartyIds(n)
	myKeygen := worker.LoadEcKeygenSavedData(pids)[0]

	doKeysign(nodes, tendermintPubKeys, keysignChs, myKeygen.ECDSAPub.Bytes(),
		libchain.GetChainIntFromId(TEST_CHAIN))
}
