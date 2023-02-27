package core

import (
	"encoding/json"
	"fmt"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/puzpuzpuz/xsync"
	"github.com/sisu-network/lib/log"

	"github.com/sisu-network/dheart/core/cache"
	"github.com/sisu-network/dheart/core/components"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/core/signer"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	p2ptypes "github.com/sisu-network/dheart/p2p/types"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/types"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	MaxWorker          = 2
	BatchSize          = 4
	MaxBatchSize       = 4
	MaxOutMsgCacheSize = 100
)

type Engine interface {
	Init() error
	AddNodes(nodes []*Node)
	AddRequest(request *types.WorkRequest) error
	OnNetworkMessage(message *p2ptypes.P2PMessage)
	ProcessNewMessage(tssMsg *commonTypes.TssMessage) error
	GetActiveWorkerCount() int
}

type EngineCallback interface {
	OnWorkKeygenFinished(result *htypes.KeygenResult)

	OnWorkSigningFinished(request *types.WorkRequest, result *htypes.KeysignResult)

	OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID)
}

// An defaultEngine is a main component for TSS signing. It takes the following roles:
// - Keep track of a list of running workers.
// - Cache message sent to a worker even before the worker starts. Please note that workers in the
//      networking performing the same work might not start at the same time.
// - Route a message to appropriate worker.
type defaultEngine struct {
	///////////////////////
	// Immutable data.
	///////////////////////
	myPid    *tss.PartyID
	myNode   *Node
	db       db.Database
	callback EngineCallback
	cm       p2p.ConnectionManager
	signer   signer.Signer
	nodes    *xsync.MapOf[string, *Node]
	config   config.TimeoutConfig

	///////////////////////
	// Mutable data. Any data change requires a lock operation.
	///////////////////////
	workers      *xsync.MapOf[string, worker.Worker]
	requestQueue *requestQueue

	// Cache all message before a worker starts
	preworkCache *cache.MessageCache
	// Cache messages during and after worker's execution.
	workCache       *cache.WorkMessageCache
	presignsManager components.AvailablePresigns
}

func NewEngine(myNode *Node, cm p2p.ConnectionManager, db db.Database, callback EngineCallback,
	privateKey ctypes.PrivKey, config config.TimeoutConfig) Engine {
	return &defaultEngine{
		myNode:          myNode,
		myPid:           myNode.PartyId,
		db:              db,
		cm:              cm,
		workers:         new(xsync.MapOf[string, worker.Worker]),
		requestQueue:    NewRequestQueue(),
		preworkCache:    cache.NewMessageCache(),
		callback:        callback,
		nodes:           new(xsync.MapOf[string, *Node]),
		signer:          signer.NewDefaultSigner(privateKey),
		presignsManager: components.NewAvailPresignManager(db),
		config:          config,
		workCache:       cache.NewWorkMessageCache(cache.MaxMessagePerNode, myNode.PartyId),
	}
}

func (engine *defaultEngine) Init() error {
	err := engine.presignsManager.Load()
	if err != nil {
		return err
	}

	return nil
}

func (engine *defaultEngine) AddNodes(nodes []*Node) {
	for _, node := range nodes {
		engine.nodes.Store(node.PeerId.String(), node)
	}
}

func (engine *defaultEngine) AddRequest(request *types.WorkRequest) error {
	if err := request.Validate(); err != nil {
		log.Error(err)
		return err
	}

	// Make sure that we know all the partyid in the request.
	for _, partyId := range request.AllParties {
		key := partyId.Id
		_, ok := engine.nodes.Load(key)
		if !ok && key != engine.myPid.Id {
			return fmt.Errorf("A party is the request cannot be found in the node list: %s", key)
		}
	}

	if engine.requestQueue.AddWork(request) {
		engine.startNextWork()
	}

	return nil
}

// startWork creates a new worker to execute a new task.
func (engine *defaultEngine) startWork(request *types.WorkRequest) {
	var w worker.Worker
	// Make a copy of myPid since the index will be changed during the TSS work.
	myPid := tss.NewPartyID(engine.myPid.Id, engine.myPid.Moniker, engine.myPid.KeyInt())

	// Create a new worker.
	switch request.WorkType {
	case types.EcKeygen, types.EdKeygen:
		w = worker.NewKeygenWorker(request, myPid, engine, engine.db, engine,
			engine.config)

	case types.EcSigning, types.EdSigning:
		w = worker.NewSigningWorker(request, myPid, engine, engine.db, engine,
			engine.config, MaxBatchSize, engine.presignsManager)
	}

	engine.workers.Store(request.WorkId, w)
	cachedMsgs := engine.preworkCache.PopAllMessages(request.WorkId, nil)
	log.Info("Starting a work with id ", request.WorkId, " with cache size ", len(cachedMsgs))

	if err := w.Start(cachedMsgs); err != nil {
		log.Error("Cannot start work error = ", err)
	}
}

// ProcessNewMessage processes new incoming tss message from network.
func (engine *defaultEngine) ProcessNewMessage(tssMsg *commonTypes.TssMessage) error {
	if tssMsg == nil {
		return nil
	}

	switch tssMsg.Type {
	case common.TssMessage_ASK_MESSAGE_REQUEST:
		if err := engine.OnAskMessage(tssMsg); err != nil {
			return err
		}

	default:
		addToCache := false

		worker, ok := engine.workers.Load(tssMsg.WorkId)
		if !ok {
			// This could be the case when a worker has not started yet. Save it to the cache.
			engine.preworkCache.AddMessage(tssMsg)
			addToCache = true
		}

		if !addToCache {
			if err := worker.ProcessNewMessage(tssMsg); err != nil {
				return fmt.Errorf("error when worker processing new message %w", err)
			}
		}
	}

	return nil
}

func (engine *defaultEngine) OnAskMessage(tssMsg *commonTypes.TssMessage) error {
	msgKey := tssMsg.AskRequestMessage.MsgKey
	keys, err := commonTypes.ExtractMessageKey(msgKey)
	if err != nil {
		return err
	}

	originalFrom := keys[1]
	signedMsg := engine.workCache.Get(originalFrom, msgKey)
	if signedMsg == nil {
		return nil
	}

	if m := signedMsg.TssMessage; !m.IsBroadcast() && m.GetTo() != tssMsg.GetFrom() {
		log.Warnf("Request from bad actor = %s, ignore it", tssMsg.GetFrom())
		return nil
	}

	var dest *tss.PartyID
	engine.nodes.Range(func(key string, node *Node) bool {
		if node.PartyId.Id == tssMsg.From {
			dest = node.PartyId
		}

		return true
	})

	if dest != nil {
		log.Verbose("Replying request message: ", msgKey, " dest = ", dest)
		engine.sendSignMessaged(signedMsg, []*tss.PartyID{dest})
	} else {
		log.Error("OnAskMessage: cannot find party id for ", signedMsg.TssMessage.To)
	}
	return nil
}

// finishWorker removes a worker from the current worker pool.
func (engine *defaultEngine) finishWorker(workId string) {
	engine.workers.Delete(workId)

	// fmt.Println
	s := fmt.Sprintf("%s finished work %s: remaining work id ", engine.myPid.Id, workId)
	engine.workers.Range(func(id string, value worker.Worker) bool {
		s += id
		return true
	})
	log.Verbosef(s)

	// Start next work
	engine.startNextWork()
}

// startNextWork gets a request from the queue (if not empty) and execute it. If there is no
// available worker, wait for one of the current worker to finish before running.
func (engine *defaultEngine) startNextWork() {
	if engine.workers.Size() >= MaxWorker {
		log.Verbosef("Max work reach, worker queue len = %d", engine.workers.Size())
		return
	}

	nextWork := engine.requestQueue.Pop()

	if nextWork == nil {
		return
	}

	engine.startWork(nextWork)
}

// Broadcast a message to everyone in a list.
func (engine *defaultEngine) BroadcastMessage(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
	if tssMessage.To == engine.myPid.Id {
		log.Error("This message should not be sent to its own node")
		return
	}

	signedMsg, err := engine.getSignedMessageBytes(tssMessage)
	if err != nil {
		log.Error("Cannot get signed message", err)
		return
	}

	// Add this to the cache if it's an update message.
	engine.cacheWorkMsg(signedMsg)
	engine.sendSignMessaged(signedMsg, pIDs)
}

// Send a message to a single destination.
func (engine *defaultEngine) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	if tssMessage.To == engine.myPid.Id {
		return
	}

	signedMsg, err := engine.getSignedMessageBytes(tssMessage)
	if err != nil {
		log.Error("Cannot get signed message", err)
		return
	}

	// Add this to the cache if it's an update message.
	engine.cacheWorkMsg(signedMsg)
	engine.sendSignMessaged(signedMsg, []*tss.PartyID{dest})
}

func (engine *defaultEngine) cacheWorkMsg(signedMsg *common.SignedMessage) {
	tssMsg := signedMsg.TssMessage
	if tssMsg.Type != commonTypes.TssMessage_UPDATE_MESSAGES {
		return
	}

	msgKey, err := tssMsg.GetMessageKey()
	if err != nil {
		return
	}

	engine.workCache.Add(msgKey, signedMsg)
}

// sendSignMessaged sends data to the network.
func (engine *defaultEngine) sendSignMessaged(signedMessage *common.SignedMessage, pIDs []*tss.PartyID) {
	bz, err := json.Marshal(signedMessage)
	if err != nil {
		log.Errorf("error when marshalling message %w", err)
		return
	}

	// Converts pids => peerIds
	peerIds := make([]peer.ID, 0, len(pIDs))
	for _, pid := range pIDs {
		if pid.Id == engine.myPid.Id {
			// Don't send to ourself.
			continue
		}

		node, ok := engine.nodes.Load(pid.Id)
		if !ok {
			log.Errorf("Cannot find node with party key %s", pid.Id)
			return
		}

		peerIds = append(peerIds, node.PeerId)
	}

	// Write to stream
	for _, peerId := range peerIds {
		engine.cm.WriteToStream(peerId, p2p.TSSProtocolID, bz)
	}
}

// getSignedMessageBytes signs a tss message and returns serialized bytes of the signed message.
func (engine *defaultEngine) getSignedMessageBytes(tssMessage *common.TssMessage) (*common.SignedMessage, error) {
	serialized, err := json.Marshal(tssMessage)
	if err != nil {
		return nil, fmt.Errorf("error when marshalling message %w", err)
	}

	signature, err := engine.signer.Sign(serialized)
	if err != nil {
		return nil, fmt.Errorf("error when signing %w", err)
	}

	signedMessage := &common.SignedMessage{
		From:       engine.myPid.Id,
		TssMessage: tssMessage,
		Signature:  signature,
	}

	return signedMessage, nil
}

// OnNetworkMessage implements P2PDataListener interface.
func (engine *defaultEngine) OnNetworkMessage(message *p2ptypes.P2PMessage) {
	_, ok := engine.nodes.Load(message.FromPeerId)
	if !ok {
		log.Error("Cannot find node from peer id ", message.FromPeerId)
		return
	}

	signedMessage := &common.SignedMessage{}
	if err := json.Unmarshal(message.Data, signedMessage); err != nil {
		log.Error("Error when unmarshal p2p message", err)
		return
	}

	tssMessage := signedMessage.TssMessage
	if tssMessage == nil {
		log.Verbose("Tss message is nil")
		return
	}

	// TODO: Check message signature here.
	if tssMessage.Type == common.TssMessage_UPDATE_MESSAGES && len(tssMessage.UpdateMessages) > 0 &&
		tssMessage.IsBroadcast() {
		engine.cacheWorkMsg(signedMessage)
	}

	if err := engine.ProcessNewMessage(tssMessage); err != nil {
		log.Error("Error when process new message", err)
	}
}

func (engine *defaultEngine) GetActiveWorkerCount() int {
	return engine.workers.Size()
}

// OnNodeNotSelected is called when this node is not selected by the leader in the election round.
func (engine *defaultEngine) OnNodeNotSelected(request *types.WorkRequest) {
	switch request.WorkType {
	case types.EcKeygen:
		// This should not happen as in keygen all nodes should be selected.

	case types.EcSigning:
		result := &htypes.KeysignResult{
			Outcome: htypes.OutcometNotSelected,
		}
		engine.callback.OnWorkSigningFinished(request, result)
	}

	// Finish this worker and start the next one (if any).
	engine.finishWorker(request.WorkId)
}

func (engine *defaultEngine) OnWorkFailed(request *types.WorkRequest) {
	// Clear all the worker's resources
	worker, ok := engine.workers.Load(request.WorkId)
	if !ok {
		log.Error("Worker " + request.WorkId + " does not exist.")
		return
	}

	culprits := worker.GetCulprits()
	engine.callback.OnWorkFailed(request, culprits)

	// Finish this worker and start the next one (if any).
	engine.finishWorker(request.WorkId)
}

func (engine *defaultEngine) GetAvailablePresigns(batchSize int, n int,
	allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
	return engine.presignsManager.GetAvailablePresigns(batchSize, n, allPids)
}

func (engine *defaultEngine) GetPresignOutputs(presignIds []string) []*ecsigning.SignatureData_OneRoundData {
	loaded, err := engine.db.LoadPresign(presignIds)
	if err != nil {
		log.Error("Cannot load presign, err =", err)
		return make([]*ecsigning.SignatureData_OneRoundData, 0)
	}

	return loaded
}

func (engine *defaultEngine) OnWorkerResult(request *types.WorkRequest, result *worker.WorkerResult) {
	switch request.WorkType {
	// Ecdsa
	case types.EcKeygen:
		engine.onEcKeygenFinished(request, worker.GetEcKeygenOutputs(result.JobResults)[0])
	case types.EcSigning:
		engine.onEcSigningFinished(request, worker.GetEcSigningOutputs(result.JobResults))

	// Eddsa
	case types.EdKeygen:
		engine.onEdKeygenFinished(request, worker.GetEdKeygenOutputs(result.JobResults)[0])
	case types.EdSigning:
		engine.onEdSigningFinished(request, worker.GetEdSigningOutputs(result.JobResults))
	default:
		log.Error("OnWorkerResult: Unknown work type ", request.WorkType)
	}

	// Finish this worker and start the next one (if any).
	engine.finishWorker(request.WorkId)
}
