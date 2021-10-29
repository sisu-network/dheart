package core

import (
	cryptoec "crypto/ecdsa"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"
	libCommon "github.com/sisu-network/tss-lib/common"

	"github.com/sisu-network/dheart/core/signer"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	htypes "github.com/sisu-network/dheart/types"
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
	MaxWorker = 2
	BatchSize = 4
)

var defaultJobTimeout = 10 * time.Minute

type EngineCallback interface {
	OnWorkKeygenFinished(result *htypes.KeygenResult)

	OnWorkPresignFinished(result *htypes.PresignResult)

	OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.SignatureData)

	OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID)
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
	presignsManager *AvailPresignManager

	presignJobTimeout time.Duration
	keygenJobTimeout  time.Duration
	signingJobTimeout time.Duration
}

func NewEngine(myNode *Node, cm p2p.ConnectionManager, db db.Database, callback EngineCallback, privateKey ctypes.PrivKey) *Engine {
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
		presignsManager:   NewAvailPresignManager(db),
		keygenJobTimeout:  defaultJobTimeout,
		signingJobTimeout: defaultJobTimeout,
		presignJobTimeout: defaultJobTimeout,
	}
}

func (engine *Engine) WithKeygenTimeout(timeout time.Duration) *Engine {
	engine.keygenJobTimeout = timeout
	return engine
}

func (engine *Engine) WithPresignTimeout(timeout time.Duration) *Engine {
	engine.presignJobTimeout = timeout
	return engine
}

func (engine *Engine) Init() {
	err := engine.presignsManager.Load()
	if err != nil {
		panic(err)
	}
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
	// Make a copy of myPid since the index will be changed during the TSS work.
	workPartyId := tss.NewPartyID(engine.myPid.Id, engine.myPid.Moniker, engine.myPid.KeyInt())

	// Create a new worker.
	switch request.WorkType {
	case types.EcdsaKeygen:
		w = ecdsa.NewKeygenWorker(BatchSize, request, workPartyId, engine, engine.db, engine, engine.keygenJobTimeout)

	case types.EcdsaPresign:
		w = ecdsa.NewPresignWorker(BatchSize, request, workPartyId, engine, engine.db, engine, engine.presignJobTimeout)

	case types.EcdsaSigning:
		w = ecdsa.NewSigningWorker(BatchSize, request, workPartyId, engine, engine.db, engine, engine.signingJobTimeout)
	}

	engine.workLock.Lock()
	engine.workers[request.WorkId] = w
	engine.workLock.Unlock()

	cachedMsgs := engine.preworkCache.PopAllMessages(request.WorkId)
	utils.LogInfo("Starting a work with id", request.WorkId, "with cache size", len(cachedMsgs))

	if err := w.Start(cachedMsgs); err != nil {
		utils.LogError("Cannot start work error", err)
	}
}

// ProcessNewMessage processes new incoming tss message from network.
func (engine *Engine) ProcessNewMessage(tssMsg *commonTypes.TssMessage) error {
	engine.workLock.RLock()
	worker := engine.getWorker(tssMsg.WorkId)
	engine.workLock.RUnlock()

	if worker != nil {
		if err := worker.ProcessNewMessage(tssMsg); err != nil {
			return fmt.Errorf("error when worker processing new message %w", err)
		}
	} else {
		if tssMsg.Type == common.TssMessage_AVAILABILITY_REQUEST {
			// TODO: Check if we still have some available workers, create a worker and respond to the
			// leader.
		} else {
			// This could be the case when a worker has not started yet. Save it to the cache.
			engine.preworkCache.AddMessage(tssMsg)
		}
	}

	return nil
}

func (engine *Engine) OnWorkKeygenFinished(request *types.WorkRequest, output []*keygen.LocalPartySaveData) {
	utils.LogInfo("Keygen finished for chain", request.Chain)
	// Save to database
	if err := engine.db.SaveKeygenData(request.Chain, request.WorkId, request.AllParties, output); err != nil {
		utils.LogError("error when saving keygen data", err)
	}

	pkX, pkY := output[0].ECDSAPub.X(), output[0].ECDSAPub.Y()
	publicKeyECDSA := cryptoec.PublicKey{
		Curve: tss.EC(),
		X:     pkX,
		Y:     pkY,
	}
	address := crypto.PubkeyToAddress(publicKeyECDSA).Hex()
	publicKeyBytes := crypto.FromECDSAPub(&publicKeyECDSA)

	// Make a callback and start next work.
	result := htypes.KeygenResult{
		Chain:       request.Chain,
		PubKeyBytes: publicKeyBytes,
		Success:     true,
		Address:     address,
	}

	engine.callback.OnWorkKeygenFinished(&result)
	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

func (engine *Engine) OnWorkPresignFinished(request *types.WorkRequest, pids []*tss.PartyID, data []*presign.LocalPresignData) {
	utils.LogInfo("Presign finished for chain", request.Chain)

	if err := engine.db.SavePresignData(request.Chain, request.WorkId, pids, data); err != nil {
		utils.LogError("error when saving presign data", err)
	}

	result := htypes.PresignResult{
		Chain:   request.Chain,
		Success: true,
	}

	engine.callback.OnWorkPresignFinished(&result)

	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

func (engine *Engine) OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.SignatureData) {
	utils.LogInfo("Signing finished for chain", request.Chain)

	engine.callback.OnWorkSigningFinished(request, data)

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
	if len(engine.workers) >= MaxWorker {
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
		return nil, fmt.Errorf("error when marshalling message %w", err)
	}

	signature, err := engine.signer.Sign(serialized)
	if err != nil {
		return nil, fmt.Errorf("error when signing %w", err)
	}

	signedMessage := &common.SignedMessage{
		TssMessage: tssMessage,
		Signature:  signature,
	}

	bz, err := json.Marshal(signedMessage)
	if err != nil {
		return nil, fmt.Errorf("error when marshalling message %w", err)
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
	if err := json.Unmarshal(message.Data, signedMessage); err != nil {
		utils.LogError("Error when unmarshal p2p message", err)
		return
	}

	tssMessage := signedMessage.TssMessage
	if tssMessage == nil {
		return
	}

	if err := engine.ProcessNewMessage(tssMessage); err != nil {
		utils.LogError("Error when process new message", err)
	}
}

func (engine *Engine) OnPreExecutionFinished(request *types.WorkRequest) {
	// TODO: implements this
}

func (engine *Engine) OnWorkFailed(request *types.WorkRequest) {
	// Clear all the worker's resources
	engine.workLock.Lock()
	worker := engine.workers[request.WorkId]
	delete(engine.workers, request.WorkId)
	engine.workLock.Unlock()

	engine.startNextWork()
	if worker == nil {
		utils.LogError("Worker " + request.WorkId + " does not exist.")
		return
	}

	culprits := worker.GetCulprits()
	go engine.callback.OnWorkFailed(request, culprits)
	worker.Stop()
}

func (engine *Engine) GetAvailablePresigns(batchSize int, n int, pids []*tss.PartyID) ([]string, []*tss.PartyID) {
	return engine.presignsManager.GetAvailablePresigns(batchSize, n, pids)
}

func (engine *Engine) GetUnavailablePresigns(sentMsgNodes map[string]*tss.PartyID, pids []*tss.PartyID) []*tss.PartyID {
	return engine.presignsManager.GetUnavailablePresigns(sentMsgNodes, pids)
}

func (engine *Engine) GetPresignOutputs(presignIds []string) []*presign.LocalPresignData {
	loaded, err := engine.db.LoadPresign(presignIds)
	if err != nil {
		utils.LogError("Cannot load presign, err =", err)
		return make([]*presign.LocalPresignData, 0)
	}

	return loaded
}
