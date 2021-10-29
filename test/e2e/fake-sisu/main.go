package main

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/joho/godotenv"
	"github.com/sisu-network/cosmos-sdk/crypto/keys/ed25519"
	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/test/e2e/fake-sisu/mock"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
)

const (
	TEST_CHAIN = "eth"
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

func resetDb(index int) {
	// reset the dev db
	database, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", "root", "password", "0.0.0.0", 3306, fmt.Sprintf("dheart%d", index)))
	if err != nil {
		panic(err)
	}
	defer database.Close()

	database.Exec("TRUNCATE TABLE keygen")
	database.Exec("TRUNCATE TABLE presign")
}

func createNodes(index int, n int, keygenCh chan *types.KeygenResult, keysignCh chan *types.KeysignResult) *MockSisuNode {
	port := 25456 + index
	heartPort := 5678 + index

	handler := ethRpc.NewServer()
	handler.RegisterName("tss", mock.NewApi(keygenCh, keysignCh))

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

	value := big.NewInt(1000000000000000000) // in wei (1 eth)
	gasLimit := uint64(21000)                // in units
	gasPrice := big.NewInt(50)

	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	var data []byte
	tx := etypes.NewTransaction(uint64(nonce), toAddress, value, gasLimit, gasPrice, data)

	return tx
}

func setPrivateKeys(nodes []*MockSisuNode) {
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
			nodes[index].client.SetPrivKey(hex.EncodeToString(encrypt), nodes[index].privKey.Type())
		}(i)

		wg.Done()
	}

	wg.Wait()
	utils.LogInfo("Done Setting private key!")
	time.Sleep(time.Second)
}

func keygen(nodes []*MockSisuNode, tendermintPubKeys []ctypes.PubKey, keygenChs []chan *types.KeygenResult) []byte {
	n := len(nodes)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	utils.LogInfo("Sending keygen.....")

	for i := 0; i < n; i++ {
		go func(index int) {
			nodes[index].client.KeyGen("Keygen0", TEST_CHAIN, tendermintPubKeys)
		}(i)
	}

	utils.LogInfo("Waiting for keygen result")

	results := make([]*types.KeygenResult, n)
	for i := 0; i < n; i++ {
		go func(index int) {
			result := <-keygenChs[index]
			results[index] = result
			wg.Done()
		}(i)
	}

	wg.Wait()
	utils.LogInfo("Done keygen!")
	time.Sleep(time.Second)

	return results[0].PubKeyBytes
}

func keysign(nodes []*MockSisuNode, tendermintPubKeys []ctypes.PubKey, keysignChs []chan *types.KeysignResult, signature []byte) {
	n := len(nodes)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	tx := generateEthTx()
	bz, err := tx.MarshalBinary()
	if err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		request := &types.KeysignRequest{
			Id:             "Keysign0",
			OutChain:       TEST_CHAIN,
			OutHash:        "Hash0",
			OutBlockHeight: 123,
			OutBytes:       bz,
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
		match := bytes.Equal(results[i].Signature, results[0].Signature)
		if !match {
			panic("Signatures do not match")
		}
	}

	signedTx, err := tx.WithSignature(etypes.NewEIP2930Signer(big.NewInt(1)), results[0].Signature)
	if err != nil {
		panic(err)
	}

	_ = signedTx
}

func main() {
	var n int
	flag.IntVar(&n, "n", 0, "Total nodes")
	flag.Parse()

	if n == 0 {
		n = 2
	}

	for i := 0; i < n; i++ {
		resetDb(i)
	}

	loadConfigEnv("../../../.env")

	keygenChs := make([]chan *types.KeygenResult, n)
	keysignChs := make([]chan *types.KeysignResult, n)
	tendermintPubKeys := make([]ctypes.PubKey, n)
	nodes := make([]*MockSisuNode, n)

	for i := 0; i < n; i++ {
		keygenChs[i] = make(chan *types.KeygenResult)
		keysignChs[i] = make(chan *types.KeysignResult)

		nodes[i] = createNodes(i, n, keygenChs[i], keysignChs[i])

		go nodes[i].server.Run()

		tendermintPubKeys[i] = nodes[i].privKey.PubKey()
	}

	time.Sleep(time.Second)

	// Set private keys
	setPrivateKeys(nodes)

	signature := keygen(nodes, tendermintPubKeys, keygenChs)
	utils.LogInfo("All keygen tasks finished")

	// Test keysign.
	keysign(nodes, tendermintPubKeys, keysignChs, signature)
	utils.LogInfo("Finished all keysign!")
}
