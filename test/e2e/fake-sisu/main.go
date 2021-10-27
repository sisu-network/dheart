package main

import (
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
	"github.com/sisu-network/cosmos-sdk/crypto/keys/secp256k1"
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
	privKey *secp256k1.PrivKey
}

func loadConfigEnv(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}
}

func resetDb(index int) {
	// reset the dev db
	database, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", "root", "password", "0.0.0.0", "3306", fmt.Sprintf("dheart%d", index)))
	if err != nil {
		panic(err)
	}
	defer database.Close()

	database.Exec("DROP TABLE keygen")
	database.Exec("DROP TABLE presign")
}

func createNodes(index int, n int, keygenCh chan types.KeygenResult, keysignCh chan *types.KeysignResult) *MockSisuNode {
	port := 25456 + index
	heartPort := 5678 + index

	handler := ethRpc.NewServer()
	handler.RegisterName("tss", mock.NewApi(keygenCh, keysignCh))

	s := mock.NewServer(handler, "0.0.0.0", uint16(port))

	client, err := mock.DialDheart(fmt.Sprintf("http://0.0.0.0:%d", heartPort))
	if err != nil {
		panic(err)
	}

	_, privKeyBytes := p2p.GetMockConnectionConfig(n, index)

	privKey := &secp256k1.PrivKey{}
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
		encrypt, err := utils.AESDEncrypt(nodes[i].privKey.Bytes(), []byte(aesKey))
		if err != nil {
			panic(err)
		}
		nodes[i].client.SetPrivKey(hex.EncodeToString(encrypt), "secp256k1")

		wg.Done()
	}

	wg.Wait()
	utils.LogInfo("Done Setting private key!")
	time.Sleep(time.Second)
}

func keygen(nodes []*MockSisuNode, tendermintPubKeys []ctypes.PubKey, keygenChs []chan types.KeygenResult) {
	n := len(nodes)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	for i := 0; i < n; i++ {
		nodes[i].client.KeyGen("Keygen0", TEST_CHAIN, tendermintPubKeys)
	}

	for i := 0; i < n; i++ {
		go func(index int) {
			<-keygenChs[index]
			wg.Done()
		}(i)
	}

	wg.Wait()
	utils.LogInfo("Done keygen!")
	time.Sleep(time.Second)
}

func keysign(nodes []*MockSisuNode, tendermintPubKeys []ctypes.PubKey, keysignChs []chan *types.KeysignResult) {
	n := len(nodes)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	for i := 0; i < n; i++ {
		tx := generateEthTx()
		bz, err := tx.MarshalBinary()
		if err != nil {
			panic(err)
		}

		request := &types.KeysignRequest{
			Id:             "Keysign0",
			OutChain:       TEST_CHAIN,
			OutHash:        "Hash0",
			OutBlockHeight: 123,
			OutBytes:       bz,
		}
		nodes[i].client.KeySign(request, tendermintPubKeys)
	}

	for i := 0; i < n; i++ {
		go func(index int) {
			<-keysignChs[index]
			wg.Done()
		}(i)
	}

	wg.Wait()
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

	keygenChs := make([]chan types.KeygenResult, n)
	keysignChs := make([]chan *types.KeysignResult, n)
	tendermintPubKeys := make([]ctypes.PubKey, n)
	nodes := make([]*MockSisuNode, n)

	for i := 0; i < n; i++ {
		keygenChs[i] = make(chan types.KeygenResult)
		keysignChs[i] = make(chan *types.KeysignResult)

		nodes[i] = createNodes(i, n, keygenChs[i], keysignChs[i])

		go nodes[i].server.Run()

		tendermintPubKeys[i] = nodes[i].privKey.PubKey()
	}

	time.Sleep(time.Second)

	// Set private keys
	setPrivateKeys(nodes)

	keygen(nodes, tendermintPubKeys, keygenChs)
	utils.LogInfo("All keygen tasks finished")

	// Test keysign.
	keysign(nodes, tendermintPubKeys, keysignChs)
	utils.LogInfo("Finished all keysign!")
}
