package main

import (
	"bytes"
	"context"

	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	libchain "github.com/sisu-network/lib/chain"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sisu-network/lib/log"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/test/e2e/fake-sisu/mock"
	"github.com/sisu-network/dheart/test/e2e/helper"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
)

const (
	TEST_CHAIN = "ganache1"
)

type MockSisuNode struct {
	server  *mock.Server
	client  *mock.DheartClient
	privKey ctypes.PrivKey
}

func loadConfigEnv(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}
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

	_, privKeyBytes := p2p.GetMockConnectionConfig(n, index, "ed25519")

	privKey := &ed25519.PrivKey{}
	err = privKey.UnmarshalAmino(privKeyBytes)
	if err != nil {
		panic(err)
	}

	return &MockSisuNode{
		server:  s,
		client:  client,
		privKey: privKey,
	}
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
		}(i)

		wg.Done()
	}

	wg.Wait()
	log.Info("Done Setting private key!")
	time.Sleep(3 * time.Second)
}

func keygen(nodes []*MockSisuNode, tendermintPubKeys []ctypes.PubKey, keygenChs []chan *types.KeygenResult) *types.KeygenResult {
	n := len(nodes)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	log.Info("Sending keygen.....")

	for i := 0; i < n; i++ {
		go func(index int) {
			nodes[index].client.KeyGen("Keygen0", "ecdsa", tendermintPubKeys)
		}(i)
	}

	log.Info("Waiting for keygen result")

	results := make([]*types.KeygenResult, n)
	for i := 0; i < n; i++ {
		go func(index int) {
			result := <-keygenChs[index]
			results[index] = result
			wg.Done()
		}(i)
	}

	wg.Wait()
	log.Info("Done keygen!")
	time.Sleep(time.Second)

	// Sanity check the results
	// Address should not be empty
	if results[0].Address == "" {
		panic(fmt.Sprintf("Address cannot be empty"))
	}

	// Pubkeybytes should be valid
	pubkeyBytes := results[0].PubKeyBytes
	_, err := crypto.UnmarshalPubkey(pubkeyBytes)
	if err != nil {
		log.Error("Failed to unmarshal pubkey")
		panic(err)
	}

	// Everyone must have the same address, pubkey bytes
	for i := range results {
		if !results[i].Success {
			panic(fmt.Sprintf("Node %d failed to generate result", i))
		}
		if results[i].Address != results[0].Address {
			panic(fmt.Sprintf("Node %d has different address %s", i, results[i].Address))
		}
		if bytes.Compare(results[i].PubKeyBytes, results[0].PubKeyBytes) != 0 {
			panic(fmt.Sprintf("Node %d has different pubkey bytes", i))
		}
	}

	log.Info("Address = ", results[0].Address)

	return results[0]
}

func keysign(nodes []*MockSisuNode, tendermintPubKeys []ctypes.PubKey, keysignChs []chan *types.KeysignResult, publicKeyBytes []byte, chainId *big.Int) []*etypes.Transaction {
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
		sigPublicKey, err := crypto.Ecrecover(hashBytes, results[0].Signatures[j])
		if err != nil {
			panic(err)
		}
		matches := bytes.Equal(sigPublicKey, publicKeyBytes)
		if !matches {
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

// deploy a signed tx to a local ganache cli
func deploySignedTx(keygenResult *types.KeygenResult, txs []*etypes.Transaction) {
	blockTime := time.Second * 3
	ethCl, err := ethclient.Dial("http://0.0.0.0:7545")
	if err != nil {
		panic(err)
	}

	var beforeTxBalance *big.Int

	log.Info("Public key address = ", keygenResult.Address)

	for {
		balance, err := ethCl.BalanceAt(context.Background(), common.HexToAddress(keygenResult.Address), nil)
		if err != nil {
			panic(err)
		}
		time.Sleep(blockTime)
		if balance.Cmp(big.NewInt(0)) != 0 {
			beforeTxBalance = balance
			break
		}
		log.Info("Balance is 0. Keep waiting...")
	}

	for _, tx := range txs {
		err = ethCl.SendTransaction(context.Background(), tx)
		if err != nil {
			panic(err)
		}
	}

	log.Info("A transaction is dispatched")
	time.Sleep(2 * blockTime)

	afterTxBalance, err := ethCl.BalanceAt(context.Background(), common.HexToAddress(keygenResult.Address), nil)
	if err != nil {
		panic(err)
	}

	log.Info("beforeTxBalance = ", beforeTxBalance)
	log.Info("afterTxBalance = ", afterTxBalance)

	if afterTxBalance.Cmp(beforeTxBalance) == 0 {
		log.Error("Before and after balance are the same", beforeTxBalance, afterTxBalance)
		panic("sender account does not change. The tx likely failed")
	}

	_, _, err = ethCl.TransactionByHash(context.Background(), txs[0].Hash())
	if err != nil {
		panic(err)
	}
}

func main() {
	var n int
	flag.IntVar(&n, "n", 0, "Total nodes")
	flag.Parse()

	if n == 0 {
		n = 2
	}

	for i := 0; i < n; i++ {
		helper.ResetDbAtPort(i, 4000)
	}

	loadConfigEnv("../../../.env")

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

	keygenResult := keygen(nodes, tendermintPubKeys, keygenChs)
	log.Info("All keygen tasks finished")

	// Test keysign.
	txs := keysign(nodes, tendermintPubKeys, keysignChs, keygenResult.PubKeyBytes, libchain.GetChainIntFromId(TEST_CHAIN))
	log.Info("Finished all keysign!")

	deploySignedTx(keygenResult, txs)
}
