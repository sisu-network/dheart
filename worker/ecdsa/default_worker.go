package ecdsa

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/sisu-network/dheart/blame"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/core/message"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/components"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
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
	// List of parties who indicate that they are available for current tss work.
	availableParties *AvailableParties
	// Cache all tss update messages when some parties start executing while this node has not.
	preExecutionCache *worker.MessageCache

	// Execution
	// threshold  int
	jobs       []*Job
	jobsLock   *sync.RWMutex
	dispatcher interfaces.MessageDispatcher
	// Current job type of this worker. In most case, it's the same as jobType. However, in some case
	// it could be different. For example, a signing work that requires presign will have curJobType
	// value equal presign.
	curJobType atomic.Value

	isExecutionStarted atomic.Value
	isStopped          atomic.Value
	keygenInput        *keygen.LocalPreParams
	presignInput       *keygen.LocalPartySaveData // output from keygen. This field is used for presign.
	signingInput       []*presign.LocalPresignData
	signingMessages    []string

	messageMonitor     components.MessageMonitor
	messageMonitorLock *sync.RWMutex

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

	blameMgr *blame.Manager

	curRound uint32

	cfg        config.TimeoutConfig
	jobTimeout time.Duration
}

func NewKeygenWorker(
	request *types.WorkRequest,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback WorkerCallback,
	cfg config.TimeoutConfig,
) worker.Worker {
	w := baseWorker(request, request.AllParties, myPid, dispatcher, db, callback, cfg, cfg.KeygenJobTimeout, 1)

	w.jobType = wTypes.EcdsaKeygen
	w.keygenInput = request.KeygenInput
	w.keygenOutputs = make([]*keygen.LocalPartySaveData, request.BatchSize)
	w.curRound = uint32(message.Keygen1)

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
	w := baseWorker(request, request.AllParties, myPid, dispatcher, db, callback, cfg, cfg.PresignJobTimeout, maxJob)

	w.jobType = wTypes.EcdsaPresign
	w.presignInput = request.PresignInput
	w.presignOutputs = make([]*presign.LocalPresignData, request.BatchSize)
	w.curRound = uint32(message.Presign1)

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
) worker.Worker {
	// TODO: The request.Pids
	w := baseWorker(request, request.AllParties, myPid, dispatcher, db, callback, cfg, cfg.SigningJobTimeout, maxJob)

	w.jobType = wTypes.EcdsaSigning
	w.signingOutputs = make([]*libCommon.SignatureData, request.BatchSize)
	w.signingMessages = request.Messages
	w.curRound = uint32(message.Sign1)

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
	jobTimeout time.Duration,
	maxJob int,
) *DefaultWorker {
	return &DefaultWorker{
		request:            request,
		workId:             request.WorkId,
		batchSize:          request.BatchSize,
		db:                 db,
		myPid:              myPid,
		pidsLock:           &sync.RWMutex{},
		allParties:         allParties,
		dispatcher:         dispatcher,
		callback:           callback,
		jobs:               make([]*Job, request.BatchSize),
		jobsLock:           &sync.RWMutex{},
		jobOutput:          make(map[string][]tss.Message),
		jobOutputLock:      &sync.RWMutex{},
		finalOutputLock:    &sync.RWMutex{},
		preExecMsgCh:       make(chan *commonTypes.PreExecOutputMessage, 1),
		memberResponseCh:   make(chan *commonTypes.TssMessage, len(allParties)),
		availableParties:   NewAvailableParties(),
		preExecutionCache:  worker.NewMessageCache(),
		blameMgr:           blame.NewManager(),
		jobTimeout:         jobTimeout,
		cfg:                cfg,
		maxJob:             maxJob,
		messageMonitorLock: &sync.RWMutex{},
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
		w.setPids(w.request.AllParties)
		go func() {
			if err := w.executeWork(w.jobType); err != nil {
				log.Error("Error when executing work", err)
				return
			}
		}()
	} else {
		go w.preExecution()
	}

	return nil
}

func (w *DefaultWorker) setPids(pids []*tss.PartyID) {
	w.pidsLock.Lock()
	w.pIDs = pids
	w.pIDsMap = pidsToMap(pids)
	w.pidsLock.Unlock()
}

func (w *DefaultWorker) getPids() []*tss.PartyID {
	w.pidsLock.RLock()
	defer w.pidsLock.RUnlock()

	return w.pIDs
}

func (w *DefaultWorker) getPidFromId(id string) *tss.PartyID {
	w.pidsLock.RLock()
	defer w.pidsLock.RUnlock()

	return w.pIDsMap[id]
}

// TODO: handle error when this executeWork fails.
// Start actual execution of the work.
func (w *DefaultWorker) executeWork(workType wTypes.WorkType) error {
	if workType == wTypes.EcdsaKeygen && w.request.KeygenInput == nil {
		if err := w.loadPreparams(); err != nil {
			return err
		}
	}

	log.Info("Executing work type ", wTypes.WorkTypeStrings[workType])
	p2pCtx := tss.NewPeerContext(w.getPids())

	// Assign the correct index for our pid.
	for _, p := range w.pIDs {
		if w.myPid.Id == p.Id {
			w.myPid.Index = p.Index
		}
	}

	params := tss.NewParameters(p2pCtx, w.myPid, len(w.getPids()), w.request.Threshold)

	jobs := make([]*Job, w.batchSize)
	log.Info("batchSize = ", w.batchSize)
	nextJobType := w.jobType
	// Creates all jobs
	for i := range jobs {
		switch w.jobType {
		case wTypes.EcdsaKeygen:
			jobs[i] = NewKeygenJob(w.workId, i, w.getPids(), params, w.keygenInput, w, w.jobTimeout)
			nextJobType = wTypes.EcdsaKeygen
			w.startMessageMonitor(nextJobType)

		case wTypes.EcdsaPresign:
			w.presignInput = w.request.PresignInput
			jobs[i] = NewPresignJob(w.workId, i, w.getPids(), params, w.presignInput, w, w.jobTimeout)
			nextJobType = wTypes.EcdsaPresign
			w.startMessageMonitor(nextJobType)

		case wTypes.EcdsaSigning:
			if w.signingInput == nil || len(w.signingInput) == 0 {
				// we have to do presign first.
				w.presignInput = w.request.PresignInput
				w.presignOutputs = make([]*presign.LocalPresignData, w.batchSize)
				w.curRound = uint32(message.Presign1)
				nextJobType = wTypes.EcdsaPresign

				jobs[i] = NewPresignJob(w.workId, i, w.getPids(), params, w.presignInput, w, w.jobTimeout)
			} else {
				w.curRound = uint32(message.Sign1)
				nextJobType = wTypes.EcdsaSigning
				jobs[i] = NewSigningJob(w.workId, i, w.getPids(), params, w.signingMessages[i], w.signingInput[i], w, w.jobTimeout)
				w.startMessageMonitor(nextJobType)
			}

		default:
			// If job type is not correct, kill the whole worker.
			w.callback.OnWorkFailed(w.request)
			return fmt.Errorf("unknown job type %d", w.jobType)
		}
	}

	log.Info("nextJobType = ", nextJobType)

	for _, job := range jobs {
		if err := job.Start(); err != nil {
			log.Critical("error when starting job", err)
			// If job cannot start, kill the whole worker.
			w.callback.OnWorkFailed(w.request)
			return err
		}
	}

	w.jobsLock.Lock()
	w.jobs = jobs
	w.jobsLock.Unlock()

	w.curJobType.Store(nextJobType)

	// Mark execution started.
	w.isExecutionStarted.Store(true)

	msgCache := w.preExecutionCache.PopAllMessages(w.workId)
	if len(msgCache) > 0 {
		log.Info(w.workId, " Cache size =", len(msgCache))
	}

	for _, msg := range msgCache {
		if msg.Type == common.TssMessage_UPDATE_MESSAGES {
			if err := w.ProcessNewMessage(msg); err != nil {
				// Message can be corrupted or from bad actor, continue to execute.
				log.Error("Error when processing new message", err)
			}
		}
	}

	return nil
}

func (w *DefaultWorker) loadPreparams() error {
	// Check if we have generated preparams
	var err error
	preparams, err := w.db.LoadPreparams()
	if err == nil {
		log.Info("Preparams found")
		w.keygenInput = preparams
	} else {
		log.Error("Failed to get preparams, err =", err)
		return err
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
		// We have completed all jobs for current round. Send the list to the dispatcher. Move the worker to next round.
		dest := msg.GetTo()
		to := ""
		if dest != nil {
			to = dest[0].Id
		}

		tssMsg, err := common.NewTssMessage(w.myPid.Id, to, w.workId, list, msg.Type())
		if err != nil {
			log.Critical("Cannot build TSS message, err", err)
			return
		}

		// atomic.StoreUint32(&w.curRound, uint32(message.NextRound(w.jobType, message.Round(w.curRound))))

		log.Verbose(w.workId, " sending message ", msg.Type(), " to ", dest)

		if dest == nil {
			// broadcast
			w.dispatcher.BroadcastMessage(w.getPids(), tssMsg)
		} else {
			w.dispatcher.UnicastMessage(dest[0], tssMsg)
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
		// TODO: Do message validation here. Make sure that the round type is correct.
		if err := w.processUpdateMessages(tssMsg); err != nil {
			return fmt.Errorf("error when processing update message %w", err)
		}
	case common.TssMessage_AVAILABILITY_REQUEST:
		if err := w.onPreExecutionRequest(tssMsg); err != nil {
			return fmt.Errorf("error when processing execution request %w", err)
		}
	case common.TssMessage_AVAILABILITY_RESPONSE:
		if err := w.onPreExecutionResponse(tssMsg); err != nil {
			return fmt.Errorf("error when processing execution response %w", err)
		}
	case common.TssMessage_PRE_EXEC_OUTPUT:
		log.Debug("Received pre exec output")
		if len(w.getPids()) == 0 {
			// This output of workParticipantCh is called only once. We do checking for pids length to
			// make sure we only send message to this channel once.
			w.preExecMsgCh <- tssMsg.PreExecOutputMessage
		}

	default:
		// Don't call callback here because this can be from bad actor/corrupted.
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
		return errors.New(fmt.Sprintf("batch size does not match %d %d", w.batchSize, len(tssMsg.UpdateMessages)))
	}

	jobType := w.curJobType.Load().(wTypes.WorkType)
	// If this is signing worker (with presign) and we are in still in the presigning phase but
	// this update mesasge is for signing round, we have to catch this message.
	if jobType == wTypes.EcdsaPresign && w.jobType == wTypes.EcdsaSigning &&
		len(tssMsg.UpdateMessages) > 0 && tssMsg.UpdateMessages[0].Round == "SignRound1Message" {
		log.Verbose("We are in presign phase, add signing message to cache: ", tssMsg.UpdateMessages[0].Round)
		w.preExecutionCache.AddMessage(tssMsg)
		return nil
	}

	// Do all message validation first before processing.
	// TODO: Add more validation here.
	msgs := make([]tss.ParsedMessage, w.batchSize)
	// Now update all messages
	w.jobsLock.RLock()
	jobs := w.jobs
	w.jobsLock.RUnlock()

	fromString := tssMsg.From
	from := helper.GetPidFromString(fromString, w.getPids())
	if from == nil {
		return errors.New("sender is nil")
	}

	for i := range jobs {
		updateMessage := tssMsg.UpdateMessages[i]
		msg, err := tss.ParseWireMessage(updateMessage.Data, from, tssMsg.IsBroadcast())
		if err != nil {
			return fmt.Errorf("error when parsing wire message %w", err)
		}

		msgRouting := tss.MessageRouting{}
		if err := json.Unmarshal(updateMessage.SerializedMessageRouting, &msgRouting); err != nil {
			return fmt.Errorf("error when unmarshal message routing %w", err)
		}

		msgs[i] = msg
	}

	// Update the message monitor
	w.messageMonitorLock.RLock()
	messageMonitor := w.messageMonitor
	w.messageMonitorLock.RUnlock()
	if messageMonitor != nil {
		messageMonitor.NewMessageReceived(msgs[0], from)
	}

	for i, j := range jobs {
		go func(id int, job *Job) {
			round, err := message.GetMsgRound(msgs[id].Content())
			if err != nil {
				log.Error("error when getting round %w", err)
				// If we cannot get msg round, blame the sender
				w.blameMgr.AddCulpritByRound(w.workId, uint32(w.curRound), []*tss.PartyID{msgs[id].GetFrom()})
				return
			}

			w.blameMgr.AddSender(w.workId, uint32(round), tssMsg.From)

			// If this is a signing worker (with presign step) and this message is a signing message
			// while we are waiting for the last presign message, add the signing message to cache.

			if err := job.processMessage(msgs[id]); err != nil {
				// Message can be from bad actor/corrupted. Save the culprits, and ignore.
				log.Error("cannot process message error", err)
				if round > 0 {
					w.blameMgr.AddCulpritByRound(w.workId, uint32(round), err.Culprits())
				} else {
					// w.blameMgr.AddCulpritByRound(w.workId, atomic.LoadUint32(&w.curRound), err.Culprits())
				}
			}
		}(i, j)
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
		log.Verbose(w.GetWorkId(), " Done!")
		w.callback.OnWorkKeygenFinished(w.request, w.keygenOutputs)
		w.finished()
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
		log.Verbose(w.GetWorkId(), " Presign Done!")

		// If this is a signing request, we have to do signing after generating presign input
		if w.jobType == types.EcdsaSigning {
			w.signingInput = w.presignOutputs

			w.executeWork(types.EcdsaSigning)
		} else {
			// This is purely presign request. Make a callback after finish
			w.callback.OnWorkPresignFinished(w.request, w.getPids(), w.presignOutputs)
			w.finished()
		}
	}
}

func (w *DefaultWorker) OnJobTimeout() {
	w.callback.OnWorkFailed(w.request)
	w.finished()
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
		log.Verbose(w.GetWorkId(), " Signing Done!")
		w.callback.OnWorkSigningFinished(w.request, w.signingOutputs)
		w.finished()
	}
}

func (w *DefaultWorker) finished() {
	if w.messageMonitor != nil {
		w.messageMonitor.Stop()
	}
}

// Overrides MessageMonitorCallback
func (w *DefaultWorker) OnMissingMesssageDetected(m map[string][]string) {
	// We have found missing messages
	for pid, msgTypes := range m {
		for _, msgType := range msgTypes {
			if message.IsBroadcastMessage(msgType) {
				msgKey := common.GetMessageKey(w.workId, pid, "", msgType)
				msg := common.NewRequestMessage(w.myPid.Id, "", w.workId, msgKey)

				w.dispatcher.BroadcastMessage(w.getPids(), msg)
			} else {
				msgKey := common.GetMessageKey(w.workId, pid, w.myPid.Id, msgType)
				msg := common.NewRequestMessage(w.myPid.Id, pid, w.workId, msgKey)

				w.dispatcher.UnicastMessage(w.getPidFromId(pid), msg)
			}
		}
	}
}

func (w *DefaultWorker) GetPartyMap() map[string]*tss.PartyID {
	w.pidsLock.RLock()
	defer w.pidsLock.RUnlock()

	return w.pIDsMap
}

// Implements GetPartyId() of Worker interface.
func (w *DefaultWorker) GetPartyId() string {
	return w.myPid.Id
}

func (w *DefaultWorker) GetWorkId() string {
	return w.workId
}

func (w *DefaultWorker) GetCulprits() []*tss.PartyID {
	// If there's pre_execution error, return pre_execution culprits
	culprits := w.blameMgr.GetPreExecutionCulprits()
	if len(culprits) > 0 {
		return culprits
	}

	// return w.blameMgr.GetRoundCulprits(w.workId, atomic.LoadUint32(&w.curRound), w.pIDsMap)
	return make([]*tss.PartyID, 0)
}

func (w *DefaultWorker) getPartyIdFromString(pid string) *tss.PartyID {
	for _, p := range w.allParties {
		if p.Id == pid {
			return p
		}
	}

	return nil
}

func (w *DefaultWorker) startMessageMonitor(jobType wTypes.WorkType) {
	w.messageMonitorLock.RLock()
	messageMonitor := w.messageMonitor
	w.messageMonitorLock.RUnlock()

	if messageMonitor != nil {
		messageMonitor.Stop()
	}

	w.messageMonitorLock.Lock()
	w.messageMonitor = components.NewMessageMonitor(w.myPid, jobType, w, w.GetPartyMap(), w.cfg.MonitorMessageTimeout)
	w.messageMonitorLock.Unlock()

	go w.messageMonitor.Start()
}
