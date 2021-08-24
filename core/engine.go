package core

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	libCommon "github.com/sisu-network/tss-lib/common"
	tcrypto "github.com/tendermint/tendermint/crypto"

	"github.com/sisu-network/dheart/core/signer"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/ecdsa"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	MAX_WORKER = 2
	BATCH_SIZE = 4
)

type EngineCallback interface {
	OnWorkKeygenFinished(workerId string, data []*keygen.LocalPartySaveData)

	OnWorkPresignFinished(workerId string, data []*presign.LocalPresignData)

	OnWorkSigningFinished(workerId string, data []*libCommon.SignatureData)
}

// An Engine is a main component for TSS signing. It takes the following roles:
// - Keep track of a list of running workers.
// - Cache message sent to a worker even before the worker starts. Please note that workers in the
//      networking performing the same work might not start at the same time.
// - Route a message to appropriate worker.
type Engine struct {
	myPid *tss.PartyID

	workers      map[string]worker.Worker
	requestQueue *requestQueue

	workLock     *sync.RWMutex
	preworkCache *PreworkMessageCache

	callback EngineCallback
	cm       p2p.ConnectionManager
	signer   signer.Signer

	nodes    map[string]*Node
	nodeLock *sync.RWMutex
}

func NewEngine(myPid *tss.PartyID, cm p2p.ConnectionManager, callback EngineCallback, privateKey tcrypto.PrivKey) *Engine {
	return &Engine{
		myPid:        myPid,
		cm:           cm,
		workers:      make(map[string]worker.Worker),
		requestQueue: NewRequestQueue(),
		workLock:     &sync.RWMutex{},
		preworkCache: NewPreworkMessageCache(),
		callback:     callback,
		nodes:        make(map[string]*Node),
		signer:       signer.NewDefaultSigner(privateKey),
		nodeLock:     &sync.RWMutex{},
	}
}

func (engine *Engine) AddNodes(nodes []*Node) {
	engine.nodeLock.Lock()
	defer engine.nodeLock.Unlock()

	for _, node := range nodes {
		engine.nodes[node.PartyId.KeyInt().String()] = node
	}
}

func (engine *Engine) AddRequest(request *types.WorkRequest) error {
	if err := request.Validate(); err != nil {
		utils.LogError(err)
		return err
	}

	for _, partyId := range request.PIDs {
		key := partyId.KeyInt().String()
		if engine.nodes[key] == nil {
			return fmt.Errorf("A party is the request cannot be found in the node list: %s", key)
		}
	}

	engine.workLock.Lock()
	if engine.requestQueue.AddWork(request) && len(engine.workers) < MAX_WORKER {
		work := engine.requestQueue.Pop()
		go engine.startWork(work)
	}
	engine.workLock.Unlock()

	return nil
}

func (engine *Engine) startWork(request *types.WorkRequest) {
	var w worker.Worker
	errCh := make(chan error)
	// Create a new worker.
	switch request.WorkType {
	case types.ECDSA_KEYGEN:
		w = ecdsa.NewKeygenWorker(request.WorkId, BATCH_SIZE, request.PIDs, engine.myPid,
			request.KeygenInput, request.Threshold, engine, errCh, engine)

	case types.ECDSA_PRESIGN:
		p2pCtx := tss.NewPeerContext(request.PIDs)
		params := tss.NewParameters(p2pCtx, engine.myPid, len(request.PIDs), len(request.PIDs)-1)

		w = ecdsa.NewPresignWorker(request.WorkId, BATCH_SIZE, request.PIDs, engine.myPid,
			params, request.PresignInput, engine, errCh, engine)

	case types.ECDSA_SIGNING:
		p2pCtx := tss.NewPeerContext(request.PIDs)
		params := tss.NewParameters(p2pCtx, engine.myPid, len(request.PIDs), len(request.PIDs)-1)
		w = ecdsa.NewSigningWorker(request.WorkId, BATCH_SIZE, request.PIDs, engine.myPid, params,
			request.Message, request.SigningInput, engine, errCh, engine)
	}

	engine.workLock.Lock()
	engine.workers[request.WorkId] = w
	engine.workLock.Unlock()

	cachedMsgs := engine.preworkCache.PopAllMessages(request.WorkId)
	utils.LogInfo("Starting a work ", request.WorkId, "with cache size", len(cachedMsgs))

	w.Start(cachedMsgs)
}

func (engine *Engine) ProcessNewMessage(tssMsg *commonTypes.TssMessage) {
	worker := engine.getWorker(tssMsg.WorkId)

	if worker != nil {
		worker.ProcessNewMessage(tssMsg)
	} else {
		// This could be the case when a worker has not started yet. Save it to the cache.
		engine.preworkCache.AddMessage(tssMsg)
	}
}

func (engine *Engine) OnWorkKeygenFinished(workerId string, data []*keygen.LocalPartySaveData) {
	// TODO: save output.
	engine.callback.OnWorkKeygenFinished(workerId, data)
}

func (engine *Engine) OnWorkPresignFinished(workerId string, data []*presign.LocalPresignData) {
	// TODO: save output.
	engine.callback.OnWorkPresignFinished(workerId, data)
}

func (engine *Engine) OnWorkSigningFinished(workerId string, data []*libCommon.SignatureData) {
	// TODO: save output.
	engine.callback.OnWorkSigningFinished(workerId, data)
}

func (engine *Engine) getNodeFromPeerId(peerId string) *Node {
	engine.nodeLock.RLock()
	defer engine.nodeLock.RUnlock()

	return engine.nodes[peerId]
}

func (engine *Engine) getWorker(workId string) worker.Worker {
	engine.workLock.RLock()
	defer engine.workLock.RUnlock()

	return engine.workers[workId]
}

// Broadcast a message to everyone in a list.
func (engine *Engine) BroadcastMessage(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
	bz, err := engine.GetSignedMessageBytes(tssMessage)
	if err != nil {
		utils.LogError("Cannot get signed message", err)
		return
	}

	engine.sendData(bz, pIDs)
}

// Send a message to a single destination.
func (engine *Engine) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	bz, err := engine.GetSignedMessageBytes(tssMessage)
	if err != nil {
		utils.LogError("Cannot get signed message", err)
		return
	}

	engine.sendData(bz, []*tss.PartyID{dest})
}

func (engine *Engine) sendData(data []byte, pIDs []*tss.PartyID) {
	// Converts pids => peerIds
	peerIds := make([]peer.ID, 0, len(pIDs))
	engine.nodeLock.RLock()
	for _, pid := range pIDs {
		node := engine.nodes[pid.KeyInt().String()]
		if node == nil {
			utils.LogError("Cannot find node with party key", pid.KeyInt().String())
			return
		}

		peerIds = append(peerIds, node.PeerId)
	}
	engine.nodeLock.RUnlock()

	// Write to stream
	for _, peerId := range peerIds {
		engine.cm.WriteToStream(peerId, p2p.PingProtocolID, data)
	}
}

func (engine *Engine) GetSignedMessageBytes(tssMessage *common.TssMessage) ([]byte, error) {
	serialized, err := json.Marshal(tssMessage)
	if err != nil {
		return nil, err
	}

	signature, err := engine.signer.Sign(serialized)
	if err != nil {
		return nil, err
	}

	signedMessage := &common.SignedMessage{
		TssMessage: tssMessage,
		Signature:  signature,
	}

	bz, err := json.Marshal(signedMessage)
	if err != nil {
		utils.LogError("Cannot marhsal signed message", err)
		return nil, err
	}

	return bz, nil
}

func (engine *Engine) OnNetworkMessage(message *p2p.P2PMessage) {
	node := engine.getNodeFromPeerId(message.FromPeerId)
	if node == nil {
		return
	}

	signedMessage := &common.SignedMessage{}
	err := json.Unmarshal(message.Data, signedMessage)
	if err != nil {
		utils.LogError("Cannot unmarshal p2p message.")
		return
	}

	tssMessage := signedMessage.TssMessage
	if tssMessage == nil {
		return
	}

	worker := engine.getWorker(tssMessage.WorkId)
	if worker == nil {
		return
	}

	worker.ProcessNewMessage(tssMessage)
}
