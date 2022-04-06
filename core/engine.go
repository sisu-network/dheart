package core

import (
	cryptoec "crypto/ecdsa"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	libchain "github.com/sisu-network/lib/chain"
	"github.com/sisu-network/lib/log"
	libCommon "github.com/sisu-network/tss-lib/common"

	"github.com/sisu-network/dheart/core/signer"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	p2ptypes "github.com/sisu-network/dheart/p2p/types"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/ecdsa"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	MaxWorker    = 2
	BatchSize    = 4
	MaxBatchSize = 4
)

var defaultJobTimeout = 10 * time.Minute

type EngineConfig struct {
	PresignJobTimeout time.Duration
	KeygenJobTimeout  time.Duration
	SigningJobTimeout time.Duration
}

// go:generate mockgen -source core/engine.go -destination=test/mock/core/engine.go -package=mock
type Engine interface {
	Init()

	AddNodes(nodes []*Node)

	AddRequest(request *types.WorkRequest) error

	OnNetworkMessage(message *p2ptypes.P2PMessage)

	ProcessNewMessage(tssMsg *commonTypes.TssMessage) error

	GetActiveWorkerCount() int
}

type EngineCallback interface {
	OnWorkKeygenFinished(result *htypes.KeygenResult)

	OnWorkPresignFinished(result *htypes.PresignResult)

	OnWorkSigningFinished(request *types.WorkRequest, result *htypes.KeysignResult)

	OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID)
}

// An DefaultEngine is a main component for TSS signing. It takes the following roles:
// - Keep track of a list of running workers.
// - Cache message sent to a worker even before the worker starts. Please note that workers in the
//      networking performing the same work might not start at the same time.
// - Route a message to appropriate worker.
type DefaultEngine struct {
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

	config EngineConfig
}

func NewEngine(myNode *Node, cm p2p.ConnectionManager, db db.Database, callback EngineCallback,
	privateKey ctypes.PrivKey, config EngineConfig) Engine {
	return &DefaultEngine{
		myNode:          myNode,
		myPid:           myNode.PartyId,
		db:              db,
		cm:              cm,
		workers:         make(map[string]worker.Worker),
		requestQueue:    NewRequestQueue(),
		workLock:        &sync.RWMutex{},
		preworkCache:    worker.NewMessageCache(),
		callback:        callback,
		nodes:           make(map[string]*Node),
		signer:          signer.NewDefaultSigner(privateKey),
		nodeLock:        &sync.RWMutex{},
		presignsManager: NewAvailPresignManager(db),
		config:          config,
	}
}

func NewDefaultEngineConfig() EngineConfig {
	return EngineConfig{
		KeygenJobTimeout:  defaultJobTimeout,
		SigningJobTimeout: defaultJobTimeout,
		PresignJobTimeout: defaultJobTimeout,
	}
}

func (engine *DefaultEngine) Init() {
	err := engine.presignsManager.Load()
	if err != nil {
		panic(err)
	}
}

func (engine *DefaultEngine) AddNodes(nodes []*Node) {
	engine.nodeLock.Lock()
	defer engine.nodeLock.Unlock()

	for _, node := range nodes {
		engine.nodes[node.PeerId.String()] = node
	}
}

func (engine *DefaultEngine) AddRequest(request *types.WorkRequest) error {
	if err := request.Validate(); err != nil {
		log.Error(err)
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
func (engine *DefaultEngine) startWork(request *types.WorkRequest) {
	var w worker.Worker
	// Make a copy of myPid since the index will be changed during the TSS work.
	workPartyId := tss.NewPartyID(engine.myPid.Id, engine.myPid.Moniker, engine.myPid.KeyInt())

	// Create a new worker.
	switch request.WorkType {
	case types.EcdsaKeygen:
		w = ecdsa.NewKeygenWorker(request, workPartyId, engine, engine.db, engine,
			engine.config.KeygenJobTimeout)

	case types.EcdsaPresign:
		w = ecdsa.NewPresignWorker(request, workPartyId, engine, engine.db, engine,
			engine.config.PresignJobTimeout, MaxBatchSize)

	case types.EcdsaSigning:
		w = ecdsa.NewSigningWorker(request, workPartyId, engine, engine.db, engine,
			engine.config.SigningJobTimeout, MaxBatchSize)
	}

	engine.workLock.Lock()
	engine.workers[request.WorkId] = w
	engine.workLock.Unlock()

	cachedMsgs := engine.preworkCache.PopAllMessages(request.WorkId)
	log.Info("Starting a work with id ", request.WorkId, " with cache size ", len(cachedMsgs))

	if err := w.Start(cachedMsgs); err != nil {
		log.Error("Cannot start work error = ", err)
	}
}

// ProcessNewMessage processes new incoming tss message from network.
func (engine *DefaultEngine) ProcessNewMessage(tssMsg *commonTypes.TssMessage) error {
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

func (engine *DefaultEngine) OnWorkKeygenFinished(request *types.WorkRequest, output []*keygen.LocalPartySaveData) {
	log.Info("Keygen finished for type ", request.KeygenType)
	// Save to database
	if err := engine.db.SaveKeygenData(request.KeygenType, request.WorkId, request.AllParties, output); err != nil {
		log.Error("error when saving keygen data", err)
	}

	pkX, pkY := output[0].ECDSAPub.X(), output[0].ECDSAPub.Y()
	publicKeyECDSA := cryptoec.PublicKey{
		Curve: tss.EC(),
		X:     pkX,
		Y:     pkY,
	}
	address := crypto.PubkeyToAddress(publicKeyECDSA).Hex()
	publicKeyBytes := crypto.FromECDSAPub(&publicKeyECDSA)

	log.Verbose("publicKeyBytes length = ", len(publicKeyBytes))

	// Make a callback and start next work.
	result := htypes.KeygenResult{
		KeyType:     request.KeygenType,
		PubKeyBytes: publicKeyBytes,
		Outcome:     htypes.OutcomeSuccess,
		Address:     address,
	}

	engine.callback.OnWorkKeygenFinished(&result)
	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

func (engine *DefaultEngine) OnWorkPresignFinished(request *types.WorkRequest, pids []*tss.PartyID,
	data []*presign.LocalPresignData) {
	log.Info("Presign finished, request.WorkId = ", request.WorkId)

	engine.presignsManager.AddPresign(request.WorkId, pids, data)

	result := htypes.PresignResult{
		Outcome: htypes.OutcomeSuccess,
	}

	engine.callback.OnWorkPresignFinished(&result)

	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

func (engine *DefaultEngine) OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.SignatureData) {
	log.Info("Signing finished for workId ", request.WorkId)

	signatures := make([][]byte, len(data))

	for i, sig := range data {
		signatures[i] = sig.Signature
		if libchain.IsETHBasedChain(request.Chains[i]) {
			signatures[i] = append(signatures[i], data[i].SignatureRecovery[0])
		}
	}

	result := &htypes.KeysignResult{
		Outcome:    htypes.OutcomeSuccess,
		Signatures: signatures,
	}

	engine.callback.OnWorkSigningFinished(request, result)

	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

// finishWorker removes a worker from the current worker pool.
func (engine *DefaultEngine) finishWorker(workId string) {
	engine.workLock.Lock()
	delete(engine.workers, workId)
	engine.workLock.Unlock()
}

// startNextWork gets a request from the queue (if not empty) and execute it. If there is no
// available worker, wait for one of the current worker to finish before running.
func (engine *DefaultEngine) startNextWork() {
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

func (engine *DefaultEngine) getNodeFromPeerId(peerId string) *Node {
	engine.nodeLock.RLock()
	defer engine.nodeLock.RUnlock()

	return engine.nodes[peerId]
}

func (engine *DefaultEngine) getWorker(workId string) worker.Worker {
	engine.workLock.RLock()
	defer engine.workLock.RUnlock()

	return engine.workers[workId]
}

// Broadcast a message to everyone in a list.
func (engine *DefaultEngine) BroadcastMessage(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
	if tssMessage.To == engine.myPid.Id {
		return
	}

	bz, err := engine.getSignedMessageBytes(tssMessage)
	if err != nil {
		log.Error("Cannot get signed message", err)
		return
	}

	engine.sendData(bz, pIDs)
}

// Send a message to a single destination.
func (engine *DefaultEngine) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	if tssMessage.To == engine.myPid.Id {
		return
	}

	bz, err := engine.getSignedMessageBytes(tssMessage)
	if err != nil {
		log.Error("Cannot get signed message", err)
		return
	}

	engine.sendData(bz, []*tss.PartyID{dest})
}

// sendData sends data to the network.
func (engine *DefaultEngine) sendData(data []byte, pIDs []*tss.PartyID) {
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
			log.Error("Cannot find node with party key", pid.Id)
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
func (engine *DefaultEngine) getSignedMessageBytes(tssMessage *common.TssMessage) ([]byte, error) {
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
func (engine *DefaultEngine) OnNetworkMessage(message *p2ptypes.P2PMessage) {
	node := engine.getNodeFromPeerId(message.FromPeerId)
	if node == nil {
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

	if err := engine.ProcessNewMessage(tssMessage); err != nil {
		log.Error("Error when process new message", err)
	}
}

func (engine *DefaultEngine) GetActiveWorkerCount() int {
	engine.workLock.RLock()
	defer engine.workLock.RUnlock()

	return len(engine.workers)
}

// OnNodeNotSelected is called when this node is not selected by the leader in the election round.
func (engine *DefaultEngine) OnNodeNotSelected(request *types.WorkRequest) {
	switch request.WorkType {
	case types.EcdsaKeygen:
		// This should not happen as in keygen all nodes should be selected.

	case types.EcdsaPresign:
		result := &htypes.PresignResult{
			Outcome: htypes.OutcometNotSelected,
		}
		engine.callback.OnWorkPresignFinished(result)

	case types.EcdsaSigning:
		result := &htypes.KeysignResult{
			Outcome: htypes.OutcometNotSelected,
		}
		engine.callback.OnWorkSigningFinished(request, result)
	}
}

func (engine *DefaultEngine) OnWorkFailed(request *types.WorkRequest) {
	// Clear all the worker's resources
	engine.workLock.Lock()
	worker := engine.workers[request.WorkId]
	delete(engine.workers, request.WorkId)
	engine.workLock.Unlock()

	engine.startNextWork()
	if worker == nil {
		log.Error("Worker " + request.WorkId + " does not exist.")
		return
	}

	culprits := worker.GetCulprits()
	go engine.callback.OnWorkFailed(request, culprits)
	worker.Stop()
}

func (engine *DefaultEngine) GetAvailablePresigns(batchSize int, n int,
	allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
	return engine.presignsManager.GetAvailablePresigns(batchSize, n, allPids)
}

func (engine *DefaultEngine) GetPresignOutputs(presignIds []string) []*presign.LocalPresignData {
	loaded, err := engine.db.LoadPresign(presignIds)
	if err != nil {
		log.Error("Cannot load presign, err =", err)
		return make([]*presign.LocalPresignData, 0)
	}

	return loaded
}
