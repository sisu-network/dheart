package ecdsa

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/core/message"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/components"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/dheart/worker/types"
	wTypes "github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

type WorkExecutionResult struct {
	Success bool
	Request *types.WorkRequest

	KeygenOutputs  []*keygen.LocalPartySaveData
	PresignOutputs []*presign.LocalPresignData
	SigningOutputs []*libCommon.SignatureData
	WorkType       types.WorkType
}

type WorkerExecutor struct {
	request    *types.WorkRequest
	workType   wTypes.WorkType
	myPid      *tss.PartyID
	pIDs       []*tss.PartyID // This pids and pIDsMap do not have lock as they should be immutable
	pIDsMap    map[string]*tss.PartyID
	dispatcher interfaces.MessageDispatcher
	db         db.Database
	cfg        config.TimeoutConfig

	// Input
	keygenInput  *keygen.LocalPreParams
	presignInput *keygen.LocalPartySaveData // output from keygen. This field is used for presign.
	signingInput []*presign.LocalPresignData

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
	callback        func(WorkExecutionResult)

	jobs           []*Job
	jobsLock       *sync.RWMutex
	messageMonitor components.MessageMonitor
}

func NewWorkerExecutor(
	request *types.WorkRequest,
	workType wTypes.WorkType,
	myPid *tss.PartyID,
	pids []*tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	db db.Database,
	callback func(WorkExecutionResult),
	cfg config.TimeoutConfig,
) *WorkerExecutor {
	pIDsMap := make(map[string]*tss.PartyID)
	for _, pid := range pids {
		pIDsMap[pid.Id] = pid
	}

	return &WorkerExecutor{
		request:         request,
		workType:        workType,
		myPid:           myPid,
		pIDs:            pids,
		pIDsMap:         pIDsMap,
		dispatcher:      dispatcher,
		db:              db,
		callback:        callback,
		jobsLock:        &sync.RWMutex{},
		jobOutputLock:   &sync.RWMutex{},
		finalOutputLock: &sync.RWMutex{},
		keygenOutputs:   make([]*keygen.LocalPartySaveData, request.BatchSize),
		presignOutputs:  make([]*presign.LocalPresignData, request.BatchSize),
		signingOutputs:  make([]*libCommon.SignatureData, request.BatchSize),
		jobOutput:       make(map[string][]tss.Message),
		cfg:             cfg,
	}
}

func (w *WorkerExecutor) Init() (err error) {
	if w.workType == wTypes.EcdsaKeygen {
		if w.request.KeygenInput == nil {
			err = w.loadPreparams()
		} else {
			w.keygenInput = w.request.KeygenInput
		}
	}

	w.messageMonitor = components.NewMessageMonitor(w.myPid, w.workType, w, w.pIDsMap, w.cfg.MonitorMessageTimeout)
	go w.messageMonitor.Start()

	return nil
}

func (w *WorkerExecutor) Run(cachedMsgs []*commonTypes.TssMessage) {
	log.Info("Executing work type ", wTypes.WorkTypeStrings[w.workType])
	p2pCtx := tss.NewPeerContext(w.pIDs)

	// Assign the correct index for our pid.
	for _, p := range w.pIDs {
		if w.myPid.Id == p.Id {
			w.myPid.Index = p.Index
		}
	}

	params := tss.NewParameters(p2pCtx, w.myPid, len(w.pIDs), w.request.Threshold)

	batchSize := w.request.BatchSize

	jobs := make([]*Job, batchSize)
	log.Info("batchSize = ", batchSize)

	log.Info("WorkerExecutor WorkType = ", w.workType)

	workId := w.request.WorkId
	// Creates all jobs
	for i := range jobs {
		switch w.workType {
		case wTypes.EcdsaKeygen:
			jobs[i] = NewKeygenJob(workId, i, w.pIDs, params, w.keygenInput, w, w.cfg.KeygenJobTimeout)

		case wTypes.EcdsaPresign:
			w.presignInput = w.request.PresignInput
			jobs[i] = NewPresignJob(workId, i, w.pIDs, params, w.presignInput, w, w.cfg.PresignJobTimeout)

		case wTypes.EcdsaSigning:
			jobs[i] = NewSigningJob(workId, i, w.pIDs, params, w.request.Messages[i], w.signingInput[i], w, w.cfg.SigningJobTimeout)

		default:
			// If job type is not correct, kill the whole worker.
			w.broadcastResult(WorkExecutionResult{
				Success: true,
			})

			log.Errorf("unknown work type %d", w.workType)
			return
		}
	}

	w.jobsLock.Lock()
	w.jobs = jobs
	w.jobsLock.Unlock()

	fmt.Println("Starting all jobs....")

	startedJobs := make([]*Job, 0)
	for _, job := range jobs {
		if err := job.Start(); err != nil {
			log.Critical("error when starting job, err = ", err)
			// If job cannot start, kill the whole worker.
			w.broadcastResult(WorkExecutionResult{
				Success: false,
			})

			break
		}
		startedJobs = append(startedJobs, job)
	}

	fmt.Println("len(startedJobs) = ", len(startedJobs))

	if len(startedJobs) != len(jobs) {
		for _, job := range startedJobs {
			job.Stop()
		}
		return
	}

	log.Info(w.myPid.Id, " ", w.request.WorkId, " Cache size =", len(cachedMsgs))

	for _, msg := range cachedMsgs {
		if msg.Type == common.TssMessage_UPDATE_MESSAGES {
			if err := w.ProcessUpdateMessage(msg); err != nil {
				// Message can be corrupted or from bad actor, continue to execute.
				log.Error("Error when processing new message", err)
			}
		}
	}
}

func (w *WorkerExecutor) loadPreparams() error {
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
func (w *WorkerExecutor) OnJobMessage(job *Job, msg tss.Message) {
	// Update the list of completed jobs for current round (in the message)
	msgKey := msg.Type()
	if !msg.IsBroadcast() {
		msgKey = msgKey + "-" + msg.GetTo()[0].Id
	}

	// Update the list of finished jobs for msgKey
	w.jobOutputLock.Lock()
	list := w.jobOutput[msgKey]
	if list == nil {
		list = make([]tss.Message, w.request.BatchSize)
	}
	list[job.index] = msg
	w.jobOutput[msgKey] = list
	// Count how many job that have completed.
	count := w.getCompletedJobCount(list)
	w.jobOutputLock.Unlock()

	if count == w.request.BatchSize {
		// We have completed all jobs for current round. Send the list to the dispatcher. Move the worker to next round.
		dest := msg.GetTo()
		to := ""
		if dest != nil {
			to = dest[0].Id
		}

		tssMsg, err := common.NewTssMessage(w.myPid.Id, to, w.request.WorkId, list, msg.Type())
		if err != nil {
			log.Critical("Cannot build TSS message, err", err)
			return
		}

		log.Verbose(w.request.WorkId, ": ", w.myPid.Id, " sending message ", msg.Type(), " to ", dest)

		if dest == nil {
			// broadcast
			w.dispatcher.BroadcastMessage(w.pIDs, tssMsg)
		} else {
			w.dispatcher.UnicastMessage(dest[0], tssMsg)
		}
	}
}

// Implements OnJobKeygenFinished of JobCallback. This function is called from a job after key
// generation finishes.
func (w *WorkerExecutor) OnJobKeygenFinished(job *Job, data *keygen.LocalPartySaveData) {
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

	if count == w.request.BatchSize {
		w.broadcastResult(WorkExecutionResult{
			Success:       true,
			KeygenOutputs: w.keygenOutputs,
		})

		w.finished()
	}
}

// Implements OnJobPresignFinished of JobCallback.
func (w *WorkerExecutor) OnJobPresignFinished(job *Job, data *presign.LocalPresignData) {
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

	if count == w.request.BatchSize {
		log.Verbose(w.request.WorkId, " Presign Done!")

		// This is purely presign request. Make a callback after finish
		w.broadcastResult(WorkExecutionResult{
			Success:        true,
			PresignOutputs: w.presignOutputs,
		})

		w.finished()
	}
}

// Implements OnJobSignFinished of JobCallback.
func (w *WorkerExecutor) OnJobSignFinished(job *Job, data *libCommon.SignatureData) {
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

	if count == w.request.BatchSize {
		log.Verbose(w.request.WorkId, " Signing Done!")

		w.broadcastResult(WorkExecutionResult{
			Success:        true,
			SigningOutputs: w.signingOutputs,
		})

		w.finished()
	}
}

// Implements MessageMonitorCallback
func (w *WorkerExecutor) OnMissingMesssageDetected(m map[string][]string) {
	workId := w.request.WorkId

	// We have found missing messages
	for pid, msgTypes := range m {
		for _, msgType := range msgTypes {
			if message.IsBroadcastMessage(msgType) {
				msgKey := common.GetMessageKey(workId, pid, "", msgType)
				msg := common.NewRequestMessage(w.myPid.Id, "", workId, msgKey)

				w.dispatcher.BroadcastMessage(w.pIDs, msg)
			} else {
				msgKey := common.GetMessageKey(workId, pid, w.myPid.Id, msgType)
				msg := common.NewRequestMessage(w.myPid.Id, pid, workId, msgKey)

				w.dispatcher.UnicastMessage(w.pIDsMap[pid], msg)
			}
		}
	}
}

func (w *WorkerExecutor) ProcessUpdateMessage(tssMsg *commonTypes.TssMessage) error {
	if tssMsg.UpdateMessages[0].Round == "KGRound2Message1" && w.myPid.Index == 0 {
		fmt.Println(w.pIDsMap[tssMsg.From].Index, "->", w.pIDsMap[w.myPid.Id].Index, ": ")
	}

	// Do all message validation first before processing.
	// TODO: Add more validation here.
	msgs := make([]tss.ParsedMessage, w.request.BatchSize)
	// Now update all messages
	w.jobsLock.RLock()
	jobs := w.jobs
	w.jobsLock.RUnlock()

	fromString := tssMsg.From
	from := helper.GetPidFromString(fromString, w.pIDs)
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
	w.messageMonitor.NewMessageReceived(msgs[0], from)

	for i, j := range jobs {
		go func(id int, job *Job) {
			_, err := message.GetMsgRound(msgs[id].Content())
			if err != nil {
				log.Error("error when getting round %w", err)
				return
			}

			if err := job.processMessage(msgs[id]); err != nil {
				log.Error("worker: cannot process message, err = ", err)
				// TODO: handle failure case here.
				return
			}
		}(i, j)
	}

	return nil
}

func (w *WorkerExecutor) Stop() {
	w.jobsLock.RLock()
	jobs := w.jobs
	w.jobsLock.RUnlock()

	for _, job := range jobs {
		if job != nil {
			job.Stop()
		}
	}
}

func (w *WorkerExecutor) OnJobTimeout() {
	w.broadcastResult(WorkExecutionResult{
		Success: false,
	})
}

func (w *WorkerExecutor) finished() {
	if w.messageMonitor != nil {
		w.messageMonitor.Stop()
	}
}

func (w *WorkerExecutor) broadcastResult(result WorkExecutionResult) {
	result.WorkType = w.workType
	w.callback(result)
}

func (w *WorkerExecutor) getCompletedJobCount(list []tss.Message) int {
	count := 0
	for _, item := range list {
		if item != nil {
			count++
		}
	}

	return count
}
