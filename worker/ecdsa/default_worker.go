package ecdsa

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
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

var (
	LeaderWaitTime              = 10 * time.Second
	PreExecutionRequestWaitTime = 3 * time.Second
)

// A callback for the caller to receive updates from this worker. We use callback instead of Go
// channel to avoid creating too many channels.
type WorkerCallback interface {
	// GetAvailablePresigns returns a list of presign output that will be used for signing. The presign's
	// party ids should match the pids params passed into the function.
	GetAvailablePresigns(batchSize int, n int, pids []*tss.PartyID) ([]string, []*tss.PartyID)

	GetPresignOutputs(presignIds []string) []*presign.LocalPresignData

	OnPreExecutionFinished(request *types.WorkRequest)

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
	jobType    wTypes.WorkType
	callback   WorkerCallback
	workId     string
	db         db.Database

	// PreExecution
	preExecMsgCh     chan *common.PreExecOutputMessage
	memberResponseCh chan *common.TssMessage
	// List of parties who indicate that they are available for current tss work.
	availableParties *AvailableParties
	// Cache all tss update messages when some parties start executing while this node has not.
	preExecutionCache *worker.MessageCache

	// Execution
	threshold  int
	jobs       []*Job
	dispatcher interfaces.MessageDispatcher

	isExecutionStarted atomic.Value
	keygenInput        *keygen.LocalPreParams
	presignInput       *keygen.LocalPartySaveData // output from keygen. This field is used for presign.
	signingInput       []*presign.LocalPresignData
	signingMessage     string

	// A map between of rounds and list of messages that have been produced. The size of the list
	// is the same as batchSize.
	//
	// key: one of the 2 values
	//      - round if a message is broadcast
	//      - round-partyId if a message is unicast
	// value: list of messages that have been produced for the round.
	jobOutput     map[string][]tss.Message
	jobOutputLock *sync.RWMutex

	keygenOutputs   []*keygen.LocalPartySaveData
	presignOutputs  []*presign.LocalPresignData
	signingOutputs  []*libCommon.SignatureData
	finalOutputLock *sync.RWMutex
}

func NewKeygenWorker(
	batchSize int,
	request *types.WorkRequest,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
) worker.Worker {
	w := baseWorker(request, batchSize, request.AllParties, myPid, dispatcher, db, callback)

	w.jobType = wTypes.ECDSA_KEYGEN
	w.keygenInput = request.KeygenInput
	w.threshold = request.Threshold
	w.keygenOutputs = make([]*keygen.LocalPartySaveData, batchSize)

	return w
}

func NewPresignWorker(
	batchSize int,
	request *types.WorkRequest,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
) worker.Worker {
	w := baseWorker(request, batchSize, request.AllParties, myPid, dispatcher, db, callback)

	w.jobType = wTypes.ECDSA_PRESIGN
	w.presignInput = request.PresignInput
	w.presignOutputs = make([]*presign.LocalPresignData, batchSize)

	return w
}

func NewSigningWorker(
	batchSize int,
	request *types.WorkRequest,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
) worker.Worker {
	// TODO: The request.Pids
	w := baseWorker(request, batchSize, request.AllParties, myPid, dispatcher, db, callback)

	w.jobType = wTypes.ECDSA_SIGNING
	w.signingOutputs = make([]*libCommon.SignatureData, batchSize)
	w.signingMessage = request.Message

	return w
}

func baseWorker(
	request *types.WorkRequest,
	batchSize int,
	allParties []*tss.PartyID,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
) *DefaultWorker {
	return &DefaultWorker{
		request:           request,
		workId:            request.WorkId,
		batchSize:         batchSize,
		db:                db,
		myPid:             myPid,
		allParties:        allParties,
		dispatcher:        dispatcher,
		callback:          callback,
		jobs:              make([]*Job, batchSize),
		jobOutput:         make(map[string][]tss.Message),
		jobOutputLock:     &sync.RWMutex{},
		finalOutputLock:   &sync.RWMutex{},
		preExecMsgCh:      make(chan *commonTypes.PreExecOutputMessage, 1),
		memberResponseCh:  make(chan *commonTypes.TssMessage, len(allParties)),
		availableParties:  NewAvailableParties(),
		preExecutionCache: worker.NewMessageCache(),
	}
}

func (w *DefaultWorker) Start(preworkCache []*commonTypes.TssMessage) error {
	for _, msg := range preworkCache {
		w.preExecutionCache.AddMessage(msg)
	}

	// Do leader election and participants selection first.
	if w.request.IsKeygen() {
		// For keygen, we skip the leader selection part since all parties need to be involved in the
		// signing process.
		w.pIDs = w.request.AllParties
		go func() {
			if err := w.executeWork(); err != nil {
				utils.LogError("Error when executing work", err)
				return
			}
		}()
	} else {
		go w.preExecution()
	}

	return nil
}

// Start actual execution of the work.
func (w *DefaultWorker) executeWork() error {
	p2pCtx := tss.NewPeerContext(w.pIDs)

	// Assign the correct
	for _, p := range w.pIDs {
		if w.myPid.Id == p.Id {
			w.myPid.Index = p.Index
		}
	}

	params := tss.NewParameters(p2pCtx, w.myPid, len(w.pIDs), w.threshold)

	// Creates all jobs
	for i := range w.jobs {
		switch w.jobType {
		case wTypes.ECDSA_KEYGEN:
			w.jobs[i] = NewKeygenJob(i, w.pIDs, params, w.keygenInput, w)

		case wTypes.ECDSA_PRESIGN:
			w.jobs[i] = NewPresignJob(i, w.pIDs, params, w.presignInput, w)

		case wTypes.ECDSA_SIGNING:
			w.jobs[i] = NewSigningJob(i, w.pIDs, params, w.signingMessage, w.signingInput[i], w)

		default:
			err := fmt.Errorf("unknown job type %d", w.jobType)
			w.callback.OnWorkFailed(w.request)
			return err
		}
	}

	for _, job := range w.jobs {
		if err := job.Start(); err != nil {
			err := fmt.Errorf("error when starting job %w", err)
			w.callback.OnWorkFailed(w.request)
			return err
		}
	}

	// Mark execution started.
	w.isExecutionStarted.Store(true)

	msgCache := w.preExecutionCache.PopAllMessages(w.workId)
	for _, msg := range msgCache {
		if msg.Type == common.TssMessage_UPDATE_MESSAGES {
			if err := w.ProcessNewMessage(msg); err != nil {
				err := fmt.Errorf("error when process new message %w", err)
				return err
			}
		}
	}

	return nil
}

// Called when there is a new message from tss-lib
func (w *DefaultWorker) OnJobMessage(job *Job, msg tss.Message) {
	// Update the list of completed jobs for current round (in the message)
	msgKey := msg.Type()
	if !msg.IsBroadcast() {
		msgKey = msgKey + "-" + msg.GetTo()[0].Id
	}

	// Update the list of finished jobs for msgKey
	w.jobOutputLock.Lock()
	list := w.jobOutput[msgKey]
	if list == nil {
		list = make([]tss.Message, w.batchSize)
	}
	list[job.index] = msg
	w.jobOutput[msgKey] = list
	// Count how many job that have completed.
	count := w.getCompletedJobCount(list)
	w.jobOutputLock.Unlock()

	if count == w.batchSize {
		// We have completed all job for current round. Send the list to the dispatcher.
		dest := msg.GetTo()
		to := ""
		if dest != nil {
			to = dest[0].Id
		}

		tssMsg, err := common.NewTssMessage(w.myPid.Id, to, w.workId, list, msg.Type())
		if err != nil {
			utils.LogCritical("Cannot build TSS message, err =", err)
			w.callback.OnWorkFailed(w.request)
			return
		}

		if dest == nil {
			// broadcast
			go w.dispatcher.BroadcastMessage(w.pIDs, tssMsg)
		} else {
			go w.dispatcher.UnicastMessage(dest[0], tssMsg)
		}
	}
}

func (w *DefaultWorker) getCompletedJobCount(list []tss.Message) int {
	count := 0
	for _, item := range list {
		if item != nil {
			count++
		}
	}

	return count
}

// Process incoming update message.
func (w *DefaultWorker) ProcessNewMessage(tssMsg *commonTypes.TssMessage) error {
	switch tssMsg.Type {
	case common.TssMessage_UPDATE_MESSAGES:
		if err := w.processUpdateMessages(tssMsg); err != nil {
			w.callback.OnWorkFailed(w.request)
			return fmt.Errorf("error when processing update message %w", err)
		}
	case common.TssMessage_AVAILABILITY_REQUEST:
		if err := w.onPreExecutionRequest(tssMsg); err != nil {
			w.callback.OnWorkFailed(w.request)
			return fmt.Errorf("error when processing execution request %w", err)
		}
	case common.TssMessage_AVAILABILITY_RESPONSE:
		if err := w.onPreExecutionResponse(tssMsg); err != nil {
			w.callback.OnWorkFailed(w.request)
			return fmt.Errorf("error when processing execution response %w", err)
		}
	case common.TssMessage_PRE_EXEC_OUTPUT:
		if len(w.pIDs) == 0 {
			// This output of workParticipantCh is called only once. We do checking for pids length to
			// make sure we only send message to this channel once.
			w.preExecMsgCh <- tssMsg.PreExecOutputMessage
		}
	default:
		w.callback.OnWorkFailed(w.request)
		return errors.New("invalid message " + tssMsg.Type.String())
	}

	return nil
}

func (w *DefaultWorker) processUpdateMessages(tssMsg *commonTypes.TssMessage) error {
	// If execution has not started, save this in the cache
	if w.isExecutionStarted.Load() != true {
		w.preExecutionCache.AddMessage(tssMsg)
		return nil
	}

	if w.batchSize != len(tssMsg.UpdateMessages) {
		return errors.New("batch size does not match")
	}

	// Do all message validation first before processing.
	msgs := make([]tss.ParsedMessage, w.batchSize)
	for i := range w.jobs {
		fromString := tssMsg.From
		from := helper.GetPidFromString(fromString, w.pIDs)
		if from == nil {
			return errors.New("sender is nil")
		}

		updateMessage := tssMsg.UpdateMessages[i]
		msg, err := tss.ParseWireMessage(updateMessage.Data, from, tssMsg.IsBroadcast())
		if err != nil {
			return fmt.Errorf("error when parsing wire message %w", err)
		}

		msgRouting := tss.MessageRouting{}
		err = json.Unmarshal(updateMessage.SerializedMessageRouting, &msgRouting)
		if err != nil {
			return fmt.Errorf("error when unmarshal message routing %w", err)
		}

		msgs[i] = msg
	}

	// Now update all messages
	for i := range w.jobs {
		go func(id int) {
			if err := w.jobs[id].processMessage(msgs[id]); err != nil {
				w.callback.OnWorkFailed(w.request)
				return
			}
		}(i)
	}

	return nil
}

// Implements OnJobKeygenFinished of JobCallback. This function is called from a job after key
// generation finishes.
func (w *DefaultWorker) OnJobKeygenFinished(job *Job, data *keygen.LocalPartySaveData) {
	w.finalOutputLock.Lock()
	w.keygenOutputs[job.index] = data
	w.finalOutputLock.Unlock()

	// Count the number of finished job.
	w.finalOutputLock.RLock()
	count := 0
	for _, item := range w.keygenOutputs {
		if item != nil {
			count++
		}
	}
	w.finalOutputLock.RUnlock()

	if count == w.batchSize {
		utils.LogVerbose(w.GetWorkId(), "Done!")
		w.callback.OnWorkKeygenFinished(w.request, w.keygenOutputs)
	}
}

// Implements OnJobPresignFinished of JobCallback.
func (w *DefaultWorker) OnJobPresignFinished(job *Job, data *presign.LocalPresignData) {
	w.finalOutputLock.Lock()
	w.presignOutputs[job.index] = data
	w.finalOutputLock.Unlock()

	// Count the number of finished job.
	w.finalOutputLock.RLock()
	count := 0
	for _, item := range w.presignOutputs {
		if item != nil {
			count++
		}
	}
	w.finalOutputLock.RUnlock()

	if count == w.batchSize {
		utils.LogVerbose(w.GetWorkId(), "Done!")
		w.callback.OnWorkPresignFinished(w.request, w.pIDs, w.presignOutputs)
	}
}

// Implements OnJobSignFinished of JobCallback.
func (w *DefaultWorker) OnJobSignFinished(job *Job, data *libCommon.SignatureData) {
	w.finalOutputLock.Lock()
	w.signingOutputs[job.index] = data
	w.finalOutputLock.Unlock()

	// Count the number of finished job.
	w.finalOutputLock.RLock()
	count := 0
	for _, item := range w.signingOutputs {
		if item != nil {
			count++
		}
	}
	w.finalOutputLock.RUnlock()

	if count == w.batchSize {
		utils.LogVerbose(w.GetWorkId(), "Done!")
		w.callback.OnWorkSigningFinished(w.request, w.signingOutputs)
	}
}

// Implements GetPartyId() of Worker interface.
func (w *DefaultWorker) GetPartyId() string {
	return w.myPid.Id
}

func (w *DefaultWorker) GetWorkId() string {
	return w.workId
}

func (w *DefaultWorker) getPartyIdFromString(pid string) *tss.PartyID {
	for _, p := range w.allParties {
		if p.Id == pid {
			return p
		}
	}

	return nil
}
