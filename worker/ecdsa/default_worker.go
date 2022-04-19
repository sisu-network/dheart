package ecdsa

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	enginecache "github.com/sisu-network/dheart/core/cache"
	corecomponents "github.com/sisu-network/dheart/core/components"
	"github.com/sisu-network/lib/log"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/dheart/worker/types"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"

	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	wTypes "github.com/sisu-network/dheart/worker/types"
)

// A callback for the caller to receive updates from this worker. We use callback instead of Go
// channel to avoid creating too many channels.
type WorkerCallback interface {
	// GetAvailablePresigns returns a list of presign output that will be used for signing. The presign's
	// party ids should match the pids params passed into the function.
	GetAvailablePresigns(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID)

	GetPresignOutputs(presignIds []string) []*presign.LocalPresignData

	OnNodeNotSelected(request *types.WorkRequest)

	OnWorkFailed(request *types.WorkRequest)

	OnWorkKeygenFinished(request *types.WorkRequest, data []*keygen.LocalPartySaveData)

	OnWorkPresignFinished(request *types.WorkRequest, selectedPids []*tss.PartyID, data []*presign.LocalPresignData)

	OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.SignatureData)
}

// Implements worker.Worker interface
type DefaultWorker struct {
	batchSize  int
	request    *types.WorkRequest
	myPid      *tss.PartyID
	allParties []*tss.PartyID
	pIDs       tss.SortedPartyIDs
	pIDsMap    map[string]*tss.PartyID
	pidsLock   *sync.RWMutex
	jobType    wTypes.WorkType
	callback   WorkerCallback
	workId     string
	db         db.Database
	maxJob     int

	// PreExecution
	preExecMsgCh     chan *common.PreExecOutputMessage
	memberResponseCh chan *common.TssMessage
	// Cache all tss update messages when some parties start executing while this node has not.
	preExecutionCache *enginecache.MessageCache

	// Execution
	jobs            []*Job
	dispatcher      interfaces.MessageDispatcher
	presignsManager corecomponents.AvailablePresigns

	// This lock controls read/write for critical state change in this default worker: preworkSelection,
	// executor, secondExecutor, curJobType.
	lock *sync.RWMutex
	// For keygen and presign works, we onnly need 1 executor. For signing work, it's possible to have
	// 2 executors (presigning first and then signing) in case we cannot find a presign set that
	// satisfies available nodes.
	preworkSelection  *PreworkSelection
	executor          *WorkerExecutor
	secondaryExecutor *WorkerExecutor
	curWorkType       wTypes.WorkType

	// Current job type of this worker. In most case, it's the same as jobType. However, in some case
	// it could be different. For example, a signing work that requires presign will have curJobType
	// value equal presign.
	isExecutionStarted atomic.Value
	isStopped          atomic.Value

	cfg config.TimeoutConfig
}

func NewKeygenWorker(
	request *types.WorkRequest,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
	cfg config.TimeoutConfig,
) worker.Worker {
	w := baseWorker(request, request.AllParties, myPid, dispatcher, db, callback, cfg, 1)

	w.jobType = wTypes.EcdsaKeygen

	return w
}

func NewPresignWorker(
	request *types.WorkRequest,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
	cfg config.TimeoutConfig,
	maxJob int,
) worker.Worker {
	w := baseWorker(request, request.AllParties, myPid, dispatcher, db, callback, cfg, maxJob)

	w.jobType = wTypes.EcdsaPresign

	return w
}

func NewSigningWorker(
	request *types.WorkRequest,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
	cfg config.TimeoutConfig,
	maxJob int,
	presignsManager corecomponents.AvailablePresigns,
) worker.Worker {
	// TODO: The request.Pids
	w := baseWorker(request, request.AllParties, myPid, dispatcher, db, callback, cfg, maxJob)

	w.jobType = wTypes.EcdsaSigning
	w.presignsManager = presignsManager

	return w
}

func baseWorker(
	request *types.WorkRequest,
	allParties []*tss.PartyID,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
	cfg config.TimeoutConfig,
	maxJob int,
) *DefaultWorker {
	preExecutionCache := enginecache.NewMessageCache()

	return &DefaultWorker{
		request:           request,
		workId:            request.WorkId,
		batchSize:         request.BatchSize,
		db:                db,
		myPid:             myPid,
		pidsLock:          &sync.RWMutex{},
		allParties:        allParties,
		dispatcher:        dispatcher,
		callback:          callback,
		jobs:              make([]*Job, request.BatchSize),
		lock:              &sync.RWMutex{},
		preExecMsgCh:      make(chan *commonTypes.PreExecOutputMessage, 1),
		memberResponseCh:  make(chan *commonTypes.TssMessage, len(allParties)),
		preExecutionCache: preExecutionCache,
		cfg:               cfg,
		maxJob:            maxJob,
	}
}

func (w *DefaultWorker) Start(preworkCache []*commonTypes.TssMessage) error {
	for _, msg := range preworkCache {
		w.preExecutionCache.AddMessage(msg)
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	// Start the selection result.
	w.preworkSelection = NewPreworkSelection(w.request, w.allParties, w.myPid, w.db,
		w.preExecutionCache, w.dispatcher, w.presignsManager, w.cfg, w.onSelectionResult)
	w.preworkSelection.Init()

	cacheMsgs := w.preExecutionCache.PopAllMessages(w.workId, commonTypes.GetPreworkSelectionMsgType())
	go w.preworkSelection.Run(cacheMsgs)

	return nil
}

func (w *DefaultWorker) onSelectionResult(result SelectionResult) {
	log.Info("Selection result: Success = ", result.Success)
	if !result.Success {
		return
	}

	if result.IsNodeExcluded {
		// We are not selected. Return result in the callback and do nothing.
		return
	}

	fmt.Println("result.SelectedPids = ", result.SelectedPids)

	// Handle success case
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.request.IsKeygen() || w.request.IsPresign() || (w.request.IsSigning() &&
		len(result.PresignIds) > 0) {
		// In this case, work type = request.WorkType
		w.curWorkType = w.request.WorkType
		w.executor = w.startExecutor(result.SelectedPids)
	} else {
		// This is the case when the request is a signing work, has enough participants but cannot find
		// an appropriate presign ids to fit them all. In this case, we need to do presign first.
		// In this case work type != request.WorkType
		w.curWorkType = wTypes.EcdsaPresign
		w.secondaryExecutor = w.startExecutor(result.SelectedPids)
	}
}

func (w *DefaultWorker) startExecutor(selectedPids []*tss.PartyID) *WorkerExecutor {
	executor := NewWorkerExecutor(w.request, w.curWorkType, w.myPid, selectedPids, w.dispatcher,
		w.db, w.onJobExecutionResult, w.cfg)
	executor.Init()

	cacheMsgs := w.preExecutionCache.PopAllMessages(w.workId, commonTypes.GetUpdateMessageType())
	go executor.Run(cacheMsgs)
	return executor
}

func (w *DefaultWorker) getPidFromId(id string) *tss.PartyID {
	w.pidsLock.RLock()
	defer w.pidsLock.RUnlock()

	return w.pIDsMap[id]
}

// Process incoming update message.
func (w *DefaultWorker) ProcessNewMessage(msg *commonTypes.TssMessage) error {
	var addToCache bool

	switch msg.Type {
	case common.TssMessage_UPDATE_MESSAGES:
		w.lock.RLock()
		curExecutor := w.executor
		if w.executor == nil && w.secondaryExecutor == nil {
			// We have not started execution yet.
			w.preExecutionCache.AddMessage(msg)
			addToCache = true
			fmt.Println("Adding to cache:", w.myPid.Id, msg.UpdateMessages[0].Round)
		} else if w.request.IsSigning() && w.curWorkType.IsPresign() && msg.IsSigningMessage() {
			// This is the case when we are still in the presign phase of a signing request but some other
			// nodes finish presign early and send us a signing message. We need to cache this message
			// for later signing execution.
			w.preExecutionCache.AddMessage(msg)
			addToCache = true
			curExecutor = w.secondaryExecutor
			fmt.Println("Adding to cache:", w.myPid.Id, msg.UpdateMessages[0].Round)
		}
		w.lock.RUnlock()

		if !addToCache && curExecutor != nil {
			curExecutor.ProcessUpdateMessage(msg)
		}

	case common.TssMessage_AVAILABILITY_REQUEST, common.TssMessage_AVAILABILITY_RESPONSE, common.TssMessage_PRE_EXEC_OUTPUT:
		w.lock.RLock()
		if w.preworkSelection == nil {
			// Add this to cache
			w.preExecutionCache.AddMessage(msg)
			addToCache = true
		}
		w.lock.RUnlock()

		if !addToCache {
			return w.preworkSelection.ProcessNewMessage(msg)
		}

	default:
		// Don't call callback here because this can be from bad actor/corrupted.
		return errors.New("invalid message " + msg.Type.String())
	}

	return nil
}

func (w *DefaultWorker) onJobExecutionResult(result WorkExecutionResult) {

}

func (w *DefaultWorker) Stop() {
}

func (w *DefaultWorker) GetCulprits() []*tss.PartyID {
	// TODO: Reimplement blame manager.
	return make([]*tss.PartyID, 0)
}

// Implements GetPartyId() of Worker interface.
func (w *DefaultWorker) GetPartyId() string {
	return w.myPid.Id
}
