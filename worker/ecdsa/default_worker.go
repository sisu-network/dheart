package ecdsa

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

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

// A callback for the caller to receive updates from this worker. We use callback instead of Go
// channel to avoid creating too many channels.
type WorkerCallback interface {
	OnWorkKeygenFinished(workId string, data []*keygen.LocalPartySaveData)

	OnWorkPresignFinished(workId string, data []*presign.LocalPresignData)

	OnWorkSigningFinished(workId string, data []*libCommon.SignatureData)
}

// Implements worker.Worker interface
type DefaultWorker struct {
	batchSize  int
	myPid      *tss.PartyID
	allParties []*tss.PartyID
	pIDs       tss.SortedPartyIDs
	p2pCtx     *tss.PeerContext
	jobType    wTypes.WorkType
	callback   WorkerCallback
	workId     string
	errCh      chan error // TODO: Do this in the callback.

	threshold  int
	jobs       []*Job
	dispatcher interfaces.MessageDispatcher

	keygenInput    *keygen.LocalPreParams
	presignInput   *keygen.LocalPartySaveData // output from keygen. This field is used for presign.
	signingInput   []*presign.LocalPresignData
	signingMessage string

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
	errCh chan error,
	callback WorkerCallback,
) worker.Worker {
	w := baseWorker(request.WorkId, batchSize, request.AllParties, request.PIDs, myPid, dispatcher, errCh, callback)

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
	errCh chan error,
	callback WorkerCallback,
) worker.Worker {
	w := baseWorker(request.WorkId, batchSize, request.AllParties, request.PIDs, myPid, dispatcher, errCh, callback)

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
	errCh chan error,
	callback WorkerCallback,
) worker.Worker {
	w := baseWorker(request.WorkId, batchSize, request.AllParties, request.PIDs, myPid, dispatcher, errCh, callback)

	w.jobType = wTypes.ECDSA_SIGNING
	w.signingInput = request.SigningInput
	w.signingOutputs = make([]*libCommon.SignatureData, batchSize)
	w.signingMessage = request.Message

	return w
}

func baseWorker(
	workId string,
	batchSize int,
	allParties []*tss.PartyID,
	pIDs tss.SortedPartyIDs,
	myPid *tss.PartyID,
	dispatcher interfaces.MessageDispatcher,
	errCh chan error,
	callback WorkerCallback,
) *DefaultWorker {
	p2pCtx := tss.NewPeerContext(pIDs)

	return &DefaultWorker{
		workId:          workId,
		batchSize:       batchSize,
		myPid:           myPid,
		allParties:      allParties,
		pIDs:            pIDs,
		p2pCtx:          p2pCtx,
		dispatcher:      dispatcher,
		errCh:           errCh,
		callback:        callback,
		jobs:            make([]*Job, batchSize),
		jobOutput:       make(map[string][]tss.Message),
		jobOutputLock:   &sync.RWMutex{},
		finalOutputLock: &sync.RWMutex{},
	}
}

func (w *DefaultWorker) Start(cachedMsgs []*commonTypes.TssMessage) error {
	params := tss.NewParameters(w.p2pCtx, w.myPid, len(w.pIDs), w.threshold)

	// Creates all jobs
	for i := range w.jobs {
		switch w.jobType {
		case wTypes.ECDSA_KEYGEN:
			w.jobs[i] = NewKeygenJob(i, w.pIDs, w.myPid, params, w.keygenInput, w)

		case wTypes.ECDSA_PRESIGN:
			w.jobs[i] = NewPresignJob(i, w.pIDs, w.myPid, params, w.presignInput, w)

		case wTypes.ECDSA_SIGNING:
			w.jobs[i] = NewSigningJob(i, w.pIDs, w.myPid, params, w.signingMessage, w.signingInput[i], w)

		default:
			return errors.New(fmt.Sprint("Unknown job type", w.jobType))
		}
	}

	// Start all job
	wg := &sync.WaitGroup{}
	wg.Add(len(w.jobs))
	for _, job := range w.jobs {
		job.Start(wg)
	}

	wg.Wait()
	for _, msg := range cachedMsgs {
		w.ProcessNewMessage(msg)
	}

	return nil
}

// findPids finds list of nodes who are available for signing this work.
func (w *DefaultWorker) findPids() {

}

func (w *DefaultWorker) onError(job *Job, err error) {
	// TODO: handle error here.
	w.errCh <- err
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
	if w.batchSize != len(tssMsg.UpdateMessages) {
		return errors.New("Batch size does not match")
	}

	// Do all message validation first before processing.
	msgs := make([]tss.ParsedMessage, w.batchSize)
	for i := range w.jobs {
		fromString := tssMsg.From
		from := helper.GetFromPid(fromString, w.pIDs)
		if from == nil {
			return errors.New("Sender is nil")
		}

		updateMessage := tssMsg.UpdateMessages[i]
		msg, err := tss.ParseWireMessage(updateMessage.Data, from, tssMsg.IsBroadcast())
		if err != nil {
			utils.LogError(err)
			return err
		}

		msgRouting := tss.MessageRouting{}
		err = json.Unmarshal(updateMessage.SerializedMessageRouting, &msgRouting)
		if err != nil {
			utils.LogError(err)
			return err
		}

		msgs[i] = msg
	}

	// Now udpate all messages
	for i, job := range w.jobs {
		go job.processMessage(msgs[i])
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
		w.callback.OnWorkKeygenFinished(w.GetWorkId(), w.keygenOutputs)
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
		w.callback.OnWorkPresignFinished(w.GetWorkId(), w.presignOutputs)
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
		w.callback.OnWorkSigningFinished(w.GetWorkId(), w.signingOutputs)
	}
}

// Implements GetPartyId() of Worker interface.
func (w *DefaultWorker) GetPartyId() string {
	return w.myPid.Id
}

func (w *DefaultWorker) GetWorkId() string {
	return w.workId
}
