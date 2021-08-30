package main

import (
	"flag"
	"math/big"
	"time"

	"os"

	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/tss"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/server"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
)

type EngineCallback struct {
	keygenDataCh  chan []*keygen.LocalPartySaveData
	presignDataCh chan []*presign.LocalPresignData
	signingDataCh chan []*libCommon.SignatureData
}

func NewEngineCallback(
	keygenDataCh chan []*keygen.LocalPartySaveData,
	presignDataCh chan []*presign.LocalPresignData,
	signingDataCh chan []*libCommon.SignatureData,
) *EngineCallback {
	return &EngineCallback{
		keygenDataCh, presignDataCh, signingDataCh,
	}
}

func (cb *EngineCallback) OnWorkKeygenFinished(workerId string, data []*keygen.LocalPartySaveData) {
	cb.keygenDataCh <- data
}

func (cb *EngineCallback) OnWorkPresignFinished(workerId string, data []*presign.LocalPresignData) {
	cb.presignDataCh <- data
}

func (cb *EngineCallback) OnWorkSigningFinished(workerId string, data []*libCommon.SignatureData) {
	cb.signingDataCh <- data
}

func initialize() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
}

func getSisuClient() *client.Client {
	url := os.Getenv("SISU_SERVER_URL")
	c := client.NewClient(url)
	return c
}

func setupApiServer() {
	c := getSisuClient()

	dheart := core.NewTutTuk()

	handler := rpc.NewServer()
	if os.Getenv("USE_ON_MEMORY") == "" {
		handler.RegisterName("tss", server.NewTssApi(dheart))
	} else {
		api := server.NewSingleNodeApi(dheart, c)
		api.Init()

		handler.RegisterName("tss", api)
	}

	s := server.NewServer(handler, "localhost", 5678)

	go c.TryDial()
	go s.Run()
}

func getSortedPartyIds(n int) tss.SortedPartyIDs {
	keys := p2p.GetAllPrivateKeys(n)
	partyIds := make([]*tss.PartyID, n)

	// Creates list of party ids
	for i := 0; i < n; i++ {
		bz := keys[i].PubKey().Bytes()
		peerId := p2p.P2PIDFromKey(keys[i])
		party := tss.NewPartyID(peerId.String(), "", new(big.Int).SetBytes(bz))
		partyIds[i] = party
	}

	return tss.SortPartyIDs(partyIds, 0)
}

func main() {
	// initialize()
	// setupApiServer()

	// c := make(chan os.Signal, 1)
	// signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	// <-c

	var index, n int
	flag.IntVar(&index, "index", 0, "listening port")
	flag.Parse()

	n = 2

	config, privateKey := p2p.GetMockConnectionConfig(n, index)
	cm, err := p2p.NewConnectionManager(config)
	if err != nil {
		panic(err)
	}
	err = cm.Start(privateKey)
	if err != nil {
		panic(err)
	}

	pids := make([]*tss.PartyID, n)
	allKeys := p2p.GetAllPrivateKeys(n)
	nodes := make([]*core.Node, n)

	// Add nodes
	privKeys := p2p.GetAllPrivateKeys(n)
	for i := 0; i < n; i++ {
		pubKey := privKeys[i].PubKey()
		node := core.NewNode(pubKey)
		nodes[i] = node
		pids[i] = node.PartyId
	}

	// Create new engine
	outCh := make(chan []*keygen.LocalPartySaveData)
	cb := NewEngineCallback(outCh, nil, nil)
	engine := core.NewEngine(nodes[index], cm, cb, allKeys[index])
	cm.AddListener(p2p.TSSProtocolID, engine)

	// Add nodes
	for i := 0; i < n; i++ {
		engine.AddNodes(nodes)
	}

	time.Sleep(time.Second * 3)

	// Add request
	workId := "keygen0"
	request := types.NewKeygenRequest(workId, n, pids, *helper.LoadPreparams(index), n-1)
	err = engine.AddRequest(request)
	if err != nil {
		panic(err)
	}

	select {
	case data := <-outCh:
		utils.LogInfo("Data length = ", len(data))
	}
}
