package main

import (
	"encoding/json"
	"flag"
	"github.com/sisu-network/dheart/types/common"
	"math/big"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/p2p"
	p2pTypes "github.com/sisu-network/dheart/p2p/types"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"
)

type SlowConnectionManager struct {
	cm p2p.ConnectionManager
}

func (scm *SlowConnectionManager) Start(privKeyBytes []byte, keyType string) error {
	return scm.cm.Start(privKeyBytes, keyType)
}

func (scm *SlowConnectionManager) WriteToStream(pID peer.ID, protocolId protocol.ID, msg []byte) error {
	signedMsg := &common.SignedMessage{}
	if err := json.Unmarshal(msg, signedMsg); err != nil {
		panic(err)
	}

	if signedMsg.TssMessage.Type != common.TssMessage_UPDATE_MESSAGES {
		log.Debug("This is not update message, process it")
		return scm.cm.WriteToStream(pID, protocolId, msg)
	}

	if signedMsg.TssMessage.To == "" {
		log.Debug("Drop broadcast msg")
		return nil
	}

	log.Debug("Everything fine. Just process it")
	return scm.cm.WriteToStream(pID, protocolId, msg)
}

func (scm *SlowConnectionManager) AddListener(protocol protocol.ID, listener p2p.P2PDataListener) {
	scm.cm.AddListener(protocol, listener)
}

func NewSlowConnectionManager(config p2pTypes.ConnectionsConfig) p2p.ConnectionManager {
	return &SlowConnectionManager{
		cm: p2p.NewConnectionManager(config),
	}
}

type EngineCallback struct {
	keygenDataCh  chan *htypes.KeygenResult
	presignDataCh chan *htypes.PresignResult
	signingDataCh chan *htypes.KeysignResult
}

func NewEngineCallback(
	keygenDataCh chan *htypes.KeygenResult,
	presignDataCh chan *htypes.PresignResult,
	signingDataCh chan *htypes.KeysignResult,
) *EngineCallback {
	return &EngineCallback{
		keygenDataCh, presignDataCh, signingDataCh,
	}
}

func (cb *EngineCallback) OnWorkKeygenFinished(result *htypes.KeygenResult) {
	cb.keygenDataCh <- result
}

func (cb *EngineCallback) OnWorkPresignFinished(result *htypes.PresignResult) {
	cb.presignDataCh <- result
}

func (cb *EngineCallback) OnWorkSigningFinished(request *types.WorkRequest, result *htypes.KeysignResult) {
}

func (cb *EngineCallback) OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID) {
}

func getSortedPartyIds(n int) tss.SortedPartyIDs {
	keys := p2p.GetAllSecp256k1PrivateKeys(n)
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
	var index, n int
	var isSlowNode bool

	flag.IntVar(&index, "index", 0, "listening port")
	flag.IntVar(&n, "n", 2, "number of nodes")
	flag.BoolVar(&isSlowNode, "is-slow", false, "Use it when testing message caching mechanism")
	flag.Parse()

	config, privateKey := p2p.GetMockSecp256k1Config(n, index)
	cm := p2p.NewConnectionManager(config)
	if isSlowNode {
		cm = NewSlowConnectionManager(config)
	}
	err := cm.Start(privateKey, "secp256k1")

	if err != nil {
		panic(err)
	}

	pids := make([]*tss.PartyID, n)
	allKeys := p2p.GetAllSecp256k1PrivateKeys(n)
	nodes := make([]*core.Node, n)

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
	outCh := make(chan *htypes.KeygenResult)
	cb := NewEngineCallback(outCh, nil, nil)
	engine := core.NewEngine(nodes[index], cm, helper.NewMockDatabase(), cb, allKeys[index], core.NewDefaultEngineConfig())
	cm.AddListener(p2p.TSSProtocolID, engine)

	// Add nodes
	for i := 0; i < n; i++ {
		engine.AddNodes(nodes)
	}

	time.Sleep(time.Second * 3)

	// Add request
	workId := "keygen0"
	request := types.NewKeygenRequest("ecdsa", workId, pids, n-1, helper.LoadPreparams(index))
	err = engine.AddRequest(request)
	if err != nil {
		panic(err)
	}

	select {
	case result := <-outCh:
		log.Info("Result ", result)
	}
}
