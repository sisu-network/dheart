package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	libCommon "github.com/sisu-network/tss-lib/common"
	tcrypto "github.com/tendermint/tendermint/crypto"

	"github.com/sisu-network/dheart/core/signer"
	"github.com/sisu-network/dheart/db"
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
	OnWorkKeygenFinished(workId string, data []*keygen.LocalPartySaveData)

	OnWorkPresignFinished(workId string, data []*presign.LocalPresignData)

	OnWorkSigningFinished(workId string, data []*libCommon.SignatureData)
}

// An Engine is a main component for TSS signing. It takes the following roles:
// - Keep track of a list of running workers.
// - Cache message sent to a worker even before the worker starts. Please note that workers in the
//      networking performing the same work might not start at the same time.
// - Route a message to appropriate worker.
type Engine struct {
	myPid  *tss.PartyID
	myNode *Node
	db     db.Database

	workers      map[string]worker.Worker
	requestQueue *requestQueue

	workLock     *sync.RWMutex
	preworkCache *worker.MessageCache

	callback EngineCallback
	cm       p2p.ConnectionManager
	signer   signer.Signer

	nodes    map[string]*Node
	nodeLock *sync.RWMutex

	// TODO: Remove used presigns after getting a match to avoid using duplicated presigns.
	availablePresigns []*common.AvailablePresign
	apLock            *sync.RWMutex
}

func NewEngine(myNode *Node, cm p2p.ConnectionManager, db db.Database, callback EngineCallback, privateKey tcrypto.PrivKey) *Engine {
	return &Engine{
		myNode:            myNode,
		myPid:             myNode.PartyId,
		db:                db,
		cm:                cm,
		workers:           make(map[string]worker.Worker),
		requestQueue:      NewRequestQueue(),
		workLock:          &sync.RWMutex{},
		preworkCache:      worker.NewMessageCache(),
		callback:          callback,
		nodes:             make(map[string]*Node),
		signer:            signer.NewDefaultSigner(privateKey),
		nodeLock:          &sync.RWMutex{},
		availablePresigns: make([]*common.AvailablePresign, 0),
		apLock:            &sync.RWMutex{},
	}
}

func (engine *Engine) Init() {
	// Initialize available presigns.
	pidStrings, workIds, indexes, err := engine.db.GetAvailablePresignShortForm()
	if err != nil {
		panic("Cannot load presigns")
	}

	engine.apLock.Lock()
	for i, pidString := range pidStrings {
		ap := &common.AvailablePresign{
			Pids:       strings.Split(pidString, ","),
			WorkId:     workIds[i],
			BatchIndex: indexes[i],
		}

		engine.availablePresigns = append(engine.availablePresigns, ap)
	}
	engine.apLock.Unlock()
}

func (engine *Engine) AddNodes(nodes []*Node) {
	engine.nodeLock.Lock()
	defer engine.nodeLock.Unlock()

	for _, node := range nodes {
		engine.nodes[node.PeerId.String()] = node
	}
}

func (engine *Engine) AddRequest(request *types.WorkRequest) error {
	if err := request.Validate(); err != nil {
		utils.LogError(err)
		return err
	}

	// Make sure that we know all the partyid in the request.
	for _, partyId := range request.AllParties {
		key := partyId.Id
		node := engine.getNodeFromPeerId(key)
		if node == nil && key != engine.myPid.Id {
			return fmt.Errorf("A party is the request cannot be found in the node list: %s", key)
		}
	}

	if engine.requestQueue.AddWork(request) {
		engine.startNextWork()
	}
	return nil
}

// startWork creates a new worker to execute a new task.
func (engine *Engine) startWork(request *types.WorkRequest) {
	var w worker.Worker
	errCh := make(chan error)

	// Make a copy of myPid since the index will be changed during the TSS work.
	workPartyId := tss.NewPartyID(engine.myPid.Id, engine.myPid.Moniker, engine.myPid.KeyInt())

	// Create a new worker.
	switch request.WorkType {
	case types.ECDSA_KEYGEN:
		w = ecdsa.NewKeygenWorker(BATCH_SIZE, request, workPartyId, engine, engine.db, errCh, engine)

	case types.ECDSA_PRESIGN:
		w = ecdsa.NewPresignWorker(BATCH_SIZE, request, workPartyId, engine, engine.db, errCh, engine)

	case types.ECDSA_SIGNING:
		w = ecdsa.NewSigningWorker(BATCH_SIZE, request, workPartyId, engine, engine.db, errCh, engine)
	}

	engine.workLock.Lock()
	engine.workers[request.WorkId] = w
	engine.workLock.Unlock()

	cachedMsgs := engine.preworkCache.PopAllMessages(request.WorkId)
	utils.LogInfo("Starting a work with id", request.WorkId, "with cache size", len(cachedMsgs))

	w.Start(cachedMsgs)
}

// ProcessNewMessage processes new incoming tss message from network.
func (engine *Engine) ProcessNewMessage(tssMsg *commonTypes.TssMessage) {
	worker := engine.getWorker(tssMsg.WorkId)

	if worker != nil {
		worker.ProcessNewMessage(tssMsg)
	} else {
		if tssMsg.Type == common.TssMessage_AVAILABILITY_REQUEST {
			// TODO: Check if we still have some available workers, create a worker and respond to the
			// leader.
		} else {
			// This could be the case when a worker has not started yet. Save it to the cache.
			engine.preworkCache.AddMessage(tssMsg)
		}
	}
}

func (engine *Engine) OnWorkKeygenFinished(request *types.WorkRequest, output []*keygen.LocalPartySaveData) {
	// Save to database
	engine.db.SaveKeygenData(request.Chain, request.WorkId, request.AllParties, output)

	// Make a callback and start next work.
	engine.callback.OnWorkKeygenFinished(request.WorkId, output)
	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

func (engine *Engine) OnWorkPresignFinished(request *types.WorkRequest, pids []*tss.PartyID, data []*presign.LocalPresignData) {
	engine.db.SavePresignData(request.Chain, request.WorkId, pids, data)

	engine.callback.OnWorkPresignFinished(request.WorkId, data)

	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

func (engine *Engine) OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.SignatureData) {
	// TODO: save output.
	engine.callback.OnWorkSigningFinished(request.WorkId, data)

	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

// finishWorker removes a worker from the current worker pool.
func (engine *Engine) finishWorker(workId string) {
	engine.workLock.Lock()
	delete(engine.workers, workId)
	engine.workLock.Unlock()
}

// startNextWork gets a request from the queue (if not empty) and execute it. If there is no
// available worker, wait for one of the current worker to finish before running.
func (engine *Engine) startNextWork() {
	engine.workLock.Lock()
	if len(engine.workers) >= MAX_WORKER {
		engine.workLock.Unlock()
		return
	}
	nextWork := engine.requestQueue.Pop()
	engine.workLock.Unlock()

	if nextWork == nil {
		return
	}

	engine.startWork(nextWork)
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
	if tssMessage.To == engine.myPid.Id {
		return
	}

	bz, err := engine.getSignedMessageBytes(tssMessage)
	if err != nil {
		utils.LogError("Cannot get signed message", err)
		return
	}

	engine.sendData(bz, pIDs)
}

// Send a message to a single destination.
func (engine *Engine) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	if tssMessage.To == engine.myPid.Id {
		return
	}

	bz, err := engine.getSignedMessageBytes(tssMessage)
	if err != nil {
		utils.LogError("Cannot get signed message", err)
		return
	}

	engine.sendData(bz, []*tss.PartyID{dest})
}

// sendData sends data to the network.
func (engine *Engine) sendData(data []byte, pIDs []*tss.PartyID) {
	// Converts pids => peerIds
	peerIds := make([]peer.ID, 0, len(pIDs))
	engine.nodeLock.RLock()
	for _, pid := range pIDs {
		if pid.Id == engine.myPid.Id {
			// Don't send to ourself.
			continue
		}

		node := engine.getNodeFromPeerId(pid.Id)

		if node == nil {
			utils.LogError("Cannot find node with party key", pid.Id)
			return
		}

		peerIds = append(peerIds, node.PeerId)
	}
	engine.nodeLock.RUnlock()

	// Write to stream
	for _, peerId := range peerIds {
		engine.cm.WriteToStream(peerId, p2p.TSSProtocolID, data)
	}
}

// getSignedMessageBytes signs a tss message and returns serialized bytes of the signed message.
func (engine *Engine) getSignedMessageBytes(tssMessage *common.TssMessage) ([]byte, error) {
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

// OnNetworkMessage implements P2PDataListener interface.
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

	engine.ProcessNewMessage(tssMessage)
}

func (engine *Engine) OnPreExecutionFinished(request *types.WorkRequest) {
	// TODO: implements this
}

func (engine *Engine) OnWorkFailed(request *types.WorkRequest) {
	// TODO: implements this
}

func (engine *Engine) GetPresignData(batchSize int, n int, pids []*tss.PartyID) []*presign.LocalPresignData {
	pidMap := make(map[string]bool)
	for _, pid := range pids {
		pidMap[pid.Id] = true
	}

	selected := make([]*common.AvailablePresign, 0)

	engine.apLock.RLock()
	remaining := make([]*common.AvailablePresign, 0, len(engine.availablePresigns))
	// Loop through all available presigns set and choose a set that is also a subset of pids.
	for i, ap := range engine.availablePresigns {
		if len(ap.Pids) != n {
			continue
		}

		ok := true
		for _, pid := range ap.Pids {
			if pidMap[pid] != true {
				ok = false
				break
			}
		}

		if ok {
			selected = append(selected, ap)
			if len(selected) == batchSize {
				// Add the rest of available presign to remaining array and set this to be the new
				// availablePresigns
				for j := i + 1; j < len(engine.availablePresigns); j++ {
					remaining = append(remaining, engine.availablePresigns[j])
				}
				engine.availablePresigns = remaining
				break
			}
		} else {
			remaining = append(remaining, ap)
		}
	}
	engine.apLock.RUnlock()

	if len(selected) == 0 {
		return make([]*presign.LocalPresignData, 0)
	}

	return engine.loadPresignData(selected)
}

// loadPresignData loads full available data from a list of work ids and batch indexes. For
// convenient, these 2 values are stored inside AvailablePresign struct.
func (engine *Engine) loadPresignData(aps []*common.AvailablePresign) []*presign.LocalPresignData {
	workIds := make([]string, len(aps))
	batchIndexes := make([]int, len(aps))

	for i, ap := range aps {
		workIds[i] = ap.WorkId
		batchIndexes[i] = ap.BatchIndex
	}

	loaded, err := engine.db.LoadPresign(workIds, batchIndexes)
	if err != nil {
		utils.LogError("Cannot load presign. err =", err)
		return make([]*presign.LocalPresignData, 0)
	}

	return loaded
}
