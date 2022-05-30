package worker

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/atomic"

	enginecache "github.com/sisu-network/dheart/core/cache"
	corecomponents "github.com/sisu-network/dheart/core/components"
	"github.com/sisu-network/lib/log"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"

	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	wTypes "github.com/sisu-network/dheart/worker/types"
)

// Implements worker.Worker interface
type DefaultWorker struct {
	///////////////////////
	// Immutable data.
	///////////////////////
	batchSize       int
	request         *types.WorkRequest
	myPid           *tss.PartyID
	allParties      []*tss.PartyID
	pIDs            tss.SortedPartyIDs
	pIDsMap         map[string]*tss.PartyID
	pidsLock        *sync.RWMutex
	jobType         wTypes.WorkType
	callback        WorkerCallback
	workId          string
	db              db.Database
	maxJob          int
	dispatcher      interfaces.MessageDispatcher
	presignsManager corecomponents.AvailablePresigns
	cfg             config.TimeoutConfig

	// PreExecution
	// Cache all tss messages when sub-components have not started.
	preExecutionCache *enginecache.MessageCache

	///////////////////////
	// Mutable data. Any data change requires a lock operation.
	///////////////////////

	// Execution
	jobs []*Job

	// This lock controls read/write for critical state change in this default worker: preworkSelection,
	// executor, secondExecutor, curJobType.
	lock *sync.RWMutex
	// For keygen and presign works, we onnly need 1 executor. For signing work, it's possible to have
	// 2 executors (presigning first and then signing) in case we cannot find a presign set that
	// satisfies available nodes.
	preworkSelection  *PreworkSelection
	executor          *WorkerExecutor
	secondaryExecutor *WorkerExecutor
	// Current job type of this worker. In most case, it's the same as jobType. However, in some case
	// it could be different. For example, a signing work that requires presign will have curJobType
	// value equal presign.
	curWorkType wTypes.WorkType

	isStopped *atomic.Bool
}

func NewKeygenWorker(
	request *types.WorkRequest,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
	cfg config.TimeoutConfig,
) Worker {
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
) Worker {
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
) Worker {
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
		preExecutionCache: preExecutionCache,
		cfg:               cfg,
		maxJob:            maxJob,
		isStopped:         atomic.NewBool(false),
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

	log.Info("Worker started for job ", w.request.WorkType)

	return nil
}

func (w *DefaultWorker) onSelectionResult(result SelectionResult) {
	log.Info("Selection result: Success = ", result.Success)
	if !result.Success {
		w.callback.OnWorkFailed(w.request)
		return
	}

	if result.IsNodeExcluded {
		log.Info("I am not selected")
		// We are not selected. Return result in the callback and do nothing.
		w.callback.OnNodeNotSelected(w.request)
		return
	}

	sortedPids := tss.SortPartyIDs(result.SelectedPids)

	// We need to load the set of presigns data
	var signingInput []*presign.LocalPresignData
	if w.request.IsSigning() && len(result.PresignIds) > 0 {
		signingInput = w.callback.GetPresignOutputs(result.PresignIds)
	}

	// Handle success case
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.request.IsKeygen() || w.request.IsPresign() || (w.request.IsSigning() &&
		len(result.PresignIds) > 0) {
		// In this case, work type = request.WorkType
		w.curWorkType = w.request.WorkType
		w.executor = w.startExecutor(sortedPids, signingInput)
	} else {
		// This is the case when the request is a signing work, has enough participants but cannot find
		// an appropriate presign ids to fit them all. In this case, we need to do presign first.
		// In this case work type != request.WorkType
		log.Info("Doing presign for a signing work")
		w.curWorkType = wTypes.EcdsaPresign
		w.secondaryExecutor = w.startExecutor(sortedPids, nil)
	}
}

func (w *DefaultWorker) startExecutor(selectedPids []*tss.PartyID, signingInput []*presign.LocalPresignData) *WorkerExecutor {
	executor := NewWorkerExecutor(w.request, w.curWorkType, w.myPid, selectedPids, w.dispatcher,
		w.db, signingInput, w.onJobExecutionResult, w.cfg)
	executor.Init()

	cacheMsgs := w.preExecutionCache.PopAllMessages(w.workId, commonTypes.GetUpdateMessageType())
	go executor.Run(cacheMsgs)
	return executor
}

func (w *DefaultWorker) startTimeoutClock() {
	var timeout time.Duration
	switch w.request.WorkType {
	case types.EcdsaKeygen:
		timeout = w.cfg.KeygenJobTimeout
	case types.EcdsaPresign:
		timeout = w.cfg.PresignJobTimeout
	case types.EcdsaSigning:
		timeout = w.cfg.SigningJobTimeout
	default:
		log.Critical("Unknown work type: ", w.request.WorkType)
		return
	}

	timeout = timeout + w.cfg.SelectionLeaderTimeout

	select {
	case <-time.After(timeout):
		w.Stop()
		return
	}
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

		if w.executor == nil && w.secondaryExecutor == nil {
			// We have not started execution yet.
			w.preExecutionCache.AddMessage(msg)
			addToCache = true
			log.Verbose("Adding to cache 1:", w.workId, w.myPid.Id, msg.UpdateMessages[0].Round)
		} else if w.request.IsSigning() && w.curWorkType.IsPresign() && msg.IsSigningMessage() {
			// This is the case when we are still in the presign phase of a signing request but some other
			// nodes finish presign early and send us a signing message. We need to cache this message
			// for later signing execution.
			w.preExecutionCache.AddMessage(msg)
			addToCache = true

			log.Verbose("Adding to cache 2:", w.workId, w.myPid.Id, msg.UpdateMessages[0].Round, w.executor)
		}

		// Read the correct executor.
		curExecutor := w.executor
		if w.request.IsSigning() && w.curWorkType.IsPresign() {
			curExecutor = w.secondaryExecutor
		}
		w.lock.RUnlock()

		if !addToCache && curExecutor != nil {
			curExecutor.ProcessUpdateMessage(msg)
		}

	case common.TssMessage_AVAILABILITY_REQUEST, common.TssMessage_AVAILABILITY_RESPONSE, common.TssMessage_PRE_EXEC_OUTPUT:
		w.lock.RLock()
		if w.preworkSelection == nil {
			// Add this to cache
			log.Verbose("PreExecution is nil, adding this message to cache, msg type = ", msg.Type)
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

func (w *DefaultWorker) onJobExecutionResult(executor *WorkerExecutor, result ExecutionResult) {
	if result.Success {
		switch executor.workType {
		case types.EcdsaKeygen, types.EddsaKeygen:
			w.callback.OnWorkerResult(w.request, &WorkerResult{
				EcKeygenData: result.KeygenOutputs,
			})

		case types.EcdsaPresign:
			if w.request.IsSigning() {
				// Load signing input from db.
				presignStrings := make([]string, len(executor.pIDs))
				for i, pid := range executor.pIDs {
					presignStrings[i] = pid.Id
				}

				// This is the finished presign phase of the signing task. Continue with the signing phase.
				w.lock.Lock()
				w.curWorkType = types.EcdsaSigning
				w.executor = w.startExecutor(executor.pIDs, result.PresignOutputs)
				w.lock.Unlock()
			} else {
				w.callback.OnWorkerResult(w.request, &WorkerResult{
					SelectedPids:  executor.pIDs,
					EcPresignData: result.PresignOutputs,
				})
			}

		case types.EcdsaSigning, types.EddsaSigning:
			w.callback.OnWorkerResult(w.request, &WorkerResult{
				EcSigningData: result.SigningOutputs,
			})
		}
	} else {
		w.callback.OnWorkFailed(w.request)
	}
}

func (w *DefaultWorker) Stop() {
	w.lock.Lock()
	defer w.lock.Unlock()

	if !w.isStopped.Load() {
		go w.preworkSelection.Stop()
		if w.executor != nil {
			go w.executor.Stop()
		} else if w.secondaryExecutor != nil {
			go w.secondaryExecutor.Stop()
		}
	}
}

func (w *DefaultWorker) GetCulprits() []*tss.PartyID {
	// TODO: Reimplement blame manager.
	return make([]*tss.PartyID, 0)
}

// Implements GetPartyId() of Worker interface.
func (w *DefaultWorker) GetPartyId() string {
	return w.myPid.Id
}
